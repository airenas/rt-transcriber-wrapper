//go:generate stringer -type=State
package handlers

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/airenas/go-app/pkg/goapp"
	"github.com/airenas/rt-transcriber-wrapper/internal/api"
	"github.com/airenas/rt-transcriber-wrapper/internal/domain"
	"github.com/oklog/ulid/v2"
)

type AudioSaver interface {
	SaveAudio(ctx context.Context, id string, data [][]byte) error
}

type State int

const (
	Listening State = iota
	Transcribing
	StoppingTranscription
)

type WordPos struct {
	Segment   int
	WordIndex int
}

type TranscriptionSession struct {
	FromAudio    int64
	ToAudio      int64
	StartSegment int
	EndSegment   int
	ID           string
	stoppingAt   time.Time
	startPos     *WordPos
}

type AudioKeeper struct {
	ID    string
	Audio [][]byte
}

type RecordSession struct {
	State         State
	Auto          bool
	Segment       int
	Transcription *TranscriptionSession
	lastCommand   *WordPos
	lock          sync.Mutex

	audioKeeper *AudioKeeper
	audioSaver  AudioSaver
	user        string

	writeFunc func(msg *api.FullResult) error

	copy_command_segment       int
	select_all_command_segment int
	stop_command_segment       int
}

func NewRecordSession(audioSaver AudioSaver, user string, writeFunc func(msg *api.FullResult) error) *RecordSession {
	return &RecordSession{State: Listening, Auto: true, Segment: 0, copy_command_segment: -1, select_all_command_segment: -1,
		lastCommand: &WordPos{-1, -1}, audioSaver: audioSaver, user: user, writeFunc: writeFunc}
}

func NewTranscriptionSession(segment int, word int) *TranscriptionSession {
	return &TranscriptionSession{StartSegment: segment, EndSegment: -1, ID: ulid.Make().String(), startPos: &WordPos{Segment: segment, WordIndex: word}}
}

func (rs *RecordSession) SaveAudio(ctx context.Context) error {
	if rs.audioKeeper != nil {
		return rs.audioSaver.SaveAudio(ctx, fmt.Sprintf("audio-%s-%s", rs.user, rs.audioKeeper.ID), rs.audioKeeper.Audio)
	}
	return nil
}

func (rs *RecordSession) KeepAudio(msg []byte) {
	rs.lock.Lock()
	defer rs.lock.Unlock()
	if rs.audioKeeper != nil {
		rs.audioKeeper.Audio = append(rs.audioKeeper.Audio, msg)
	}
}

func (rs *RecordSession) Start(auto bool) {
	rs.lock.Lock()
	defer rs.lock.Unlock()
	goapp.Log.Debug().Bool("auto", auto).Msg("Starting transcription")
	rs.State = Transcribing
	rs.Auto = auto
	rs.Transcription = NewTranscriptionSession(rs.Segment, 0)
	rs.audioKeeper = &AudioKeeper{ID: rs.Transcription.ID}
}

func (rs *RecordSession) Stop(ctx context.Context) {
	rs.lock.Lock()
	defer rs.lock.Unlock()

	if rs.audioKeeper != nil {
		if err := rs.SaveAudio(ctx); err != nil {
			goapp.Log.Error().Err(err).Msg("can't save audio")
		}
		rs.audioKeeper = nil
	}
	if rs.State == Transcribing {
		rs.State = StoppingTranscription
		if rs.Transcription != nil {
			rs.Transcription.stoppingAt = time.Now()
			rs.Transcription.EndSegment = rs.Segment
			go rs.FinalStop(rs.Transcription.ID)
		}
	}
}

func (rs *RecordSession) FinalStop(id string) {
	time.Sleep(2 * time.Second)

	goapp.Log.Debug().Str("id", id).Msg("Final stopping transcription")
	rs.lock.Lock()
	defer rs.lock.Unlock()

	if id != rs.Transcription.ID {
		return
	}

	rs.State = Listening
	err := rs.writeFunc(&api.FullResult{Event: api.EventStop})
	if err != nil {
		goapp.Log.Error().Err(err).Msg("can't send stop event")
	}
}

func getText(input []*domain.K2Word) string {
	res := strings.Builder{}
	if input == nil || len(input) == 0 {
		return ""
	}
	for _, w := range input {
		if res.Len() > 0 {
			res.WriteString(" ")
		}
		res.WriteString(w.Text)
	}
	return res.String()
}

func (rs *RecordSession) Process(ctx context.Context, input *domain.K2Data, handler Handler) ([]*api.FullResult, error) {
	rs.lock.Lock()
	defer rs.lock.Unlock()
	goapp.Log.Trace().Int("segment", input.Segment).Str("txt", getText(input.NewWords)).Str("state", rs.State.String()).
		Interface("last_command", rs.lastCommand).Send()
	rs.Segment = input.Segment
	lastCommand := rs.lastCommand

	if rs.State != Transcribing && !rs.Auto {
		return nil, nil
	}
	res := []*api.FullResult{}

	if rs.State == Listening && rs.Auto {
		indexStart := startAtPos(input.NewWords, lastCommand)
		if indexStart >= 0 {
			rs.lastCommand = &WordPos{Segment: rs.Segment, WordIndex: indexStart}
			rs.State = Transcribing
			rs.Transcription = NewTranscriptionSession(rs.Segment, indexStart)
			rs.audioKeeper = &AudioKeeper{ID: rs.Transcription.ID}
			res = append(res, &api.FullResult{Event: api.EventStart, TranscriptionID: rs.Transcription.ID})
		} else {
			found := false
			if rs.copy_command_segment < rs.Segment {
				index := posAt(input.NewWords, rs.lastCommand, [][][]string{{{"kopijuoti", "kopijuok"}, {"tekstą"}}})
				if index >= 0 {
					rs.lastCommand = &WordPos{Segment: rs.Segment, WordIndex: index}

					res = append(res, &api.FullResult{Event: "COPY_COMMAND"})
					rs.copy_command_segment = rs.Segment
					found = true
				}
			}
			if !found && rs.select_all_command_segment < rs.Segment {
				index := posAt(input.NewWords, rs.lastCommand, [][][]string{{{"pažymėti", "pažymėk"}, {"visus"}}})
				if index >= 0 {
					rs.lastCommand = &WordPos{Segment: rs.Segment, WordIndex: index}

					res = append(res, &api.FullResult{Event: "SELECT_ALL_COMMAND"})
					rs.select_all_command_segment = rs.Segment
					found = true
				}
			}
			if !found && rs.stop_command_segment < rs.Segment {
				goapp.Log.Trace().Str("txt", getText(input.NewWords)).Msg("Checking stop command")
				index := posAt(input.NewWords, rs.lastCommand, [][][]string{{{"stabdyti", "stabdyk", "baik"}, {"klausymą", "klausyti"}}, {{"baiklausyti", "baiklausyte"}}})
				goapp.Log.Trace().Int("index", index).Msg("Checking stop command index")
				if index >= 0 {
					rs.lastCommand = &WordPos{Segment: rs.Segment, WordIndex: index}

					res = append(res, &api.FullResult{Event: "STOP_LISTENING_COMMAND"})
					rs.select_all_command_segment = rs.Segment
					// found = true
				}
			}
		}
	} else if rs.State == Transcribing && rs.Auto {
		indexStop := stopAtPos(input.NewWords, lastCommand)
		if indexStop >= 0 {
			rs.State = StoppingTranscription
			rs.lastCommand = &WordPos{Segment: rs.Segment, WordIndex: indexStop}
			if rs.audioKeeper != nil {
				if err := rs.SaveAudio(ctx); err != nil {
					goapp.Log.Error().Err(err).Msg("can't save audio")
				}
				rs.audioKeeper = nil
			}
			if rs.Transcription != nil {
				rs.Transcription.EndSegment = rs.Segment
				rs.Transcription.stoppingAt = time.Now()
				go rs.FinalStop(rs.Transcription.ID)
			}
			res = append(res, &api.FullResult{Event: api.EventStopping})
		}
	}

	nextState := rs.State
	if rs.State == StoppingTranscription && (rs.Transcription != nil && rs.Transcription.stoppingAt.Add(time.Second*2).After(time.Now())) {
		nextState = Listening
		res = append(res, &api.FullResult{Event: api.EventStop})

	}
	if rs.State == Listening && (rs.Transcription == nil || rs.Transcription.EndSegment < rs.Segment) {
		return res, nil
	}
	if rs.Transcription != nil && rs.Transcription.StartSegment == rs.Segment && rs.Auto {
		indexStart := startAtPos(input.NewWords, rs.Transcription.startPos)
		if indexStart >= 0 {
			input = clearWordsFrom(input, indexStart+2)
		}
	}
	if rs.Transcription != nil && rs.Transcription.EndSegment == rs.Segment && rs.Auto {
		indexStop := stopAtPos(input.NewWords, lastCommand)
		if indexStop >= 0 {
			input = clearWordsTo(input, indexStop)
		}
	}

	inputProcessed, err := handler.Process(ctx, input)
	if err != nil {
		return nil, err
	}
	newFinal := newFinal(inputProcessed)
	if newFinal > inputProcessed.FinalTo {
		out := mapToRes(inputProcessed, inputProcessed.FinalTo, newFinal, true)
		res = append(res, out)
		inputProcessed.FinalTo = newFinal
	}
	out := mapToRes(inputProcessed, inputProcessed.FinalTo, newFinal, false)
	res = append(res, out)
	rs.State = nextState
	return res, nil
}

func mapToRes(inputProcessed *domain.K2Data, from, to int, final bool) *api.FullResult {
	res :=
		&api.FullResult{
			Event:   "TRANSCRIPTION",
			Segment: inputProcessed.Segment,
			Result: api.Result{
				Hypotheses: []api.Hypothesis{
					{Transcript: getTextPunctuated(inputProcessed.Words[from:to])},
				},
				Final: final,
			},
		}
	return res
}

func getTextPunctuated(k2Word []*domain.K2Word) string {
	res := strings.Builder{}
	for _, w := range k2Word {
		if w.Punctuated != "" {
			if res.Len() > 0 {
				res.WriteString(" ")
			}
			res.WriteString(w.Punctuated)
		}
	}
	return res.String()
}

func newFinal(inputProcessed *domain.K2Data) int {
	f := inputProcessed.FinalTo
	l := len(inputProcessed.Words)
	for i := f; i < l; i++ {
		if l-i > 40 {
			f = i
		} else if l-i < 20 { // less than twenty words left, we can stop
			break
		} else {
			if inputProcessed.Words[i].SentenceEnd {
				f = i
				break
			}
		}
	}
	return f
}

func clearWordsTo(input *domain.K2Data, indexStop int) *domain.K2Data {
	ln := len(input.NewWords)
	l := len(input.Words)
	if indexStop < len(input.Words) {
		input.NewWords = input.NewWords[:indexStop]
		input.Words = input.Words[:ln-l+indexStop]
	}
	return input
}

func clearWordsFrom(input *domain.K2Data, i int) *domain.K2Data {
	ln := len(input.NewWords)
	l := len(input.Words)
	res := input
	res.NewWords = res.NewWords[i:]
	cpw := res.Words
	res.Words = res.Words[:ln-l]
	res.Words = append(res.Words, cpw[ln-l+i:]...)
	return res
}

func stopAtPos(input []*domain.K2Word, lastCommand *WordPos) int {
	return posAt(input, lastCommand, [][][]string{{{"baigiu", "baigiau", "baigiame", "baigėme", "baigti", "baik", "stabdyk", "stabdyti"}, {"įrašinėti", "įrašą", "rašinėti", "rašyti", "rašymą", "įrašymą"}},
		{{"baikrašyti", "baikrašytė"}}})
}

func startAtPos(input []*domain.K2Word, lastCommand *WordPos) int {
	return posAt(input, lastCommand, [][][]string{{{"pradedu", "pradėti", "pradedame", "pradėk"}, {"įrašinėti", "įrašą", "rašinėti", "rašyti", "rašymą", "įrašymą"}}})
}

func posAt(input []*domain.K2Word, lastCommand *WordPos, matches [][][]string) int {
	if len(input) == 0 {
		return -1
	}
	var words []string
	words = make([]string, 0, len(input))
	for _, w := range input {
		words = append(words, strings.ToLower(w.Text))
	}
	from := 0
	if lastCommand.Segment == input[0].Segment {
		from = lastCommand.WordIndex
	}
	return posInWords(words, from, matches)
}

func posInWords(words []string, from int, matches [][][]string) int {
	l := len(words)
	for _, match := range matches {
		lm := len(match)
		for i := from; i < l-lm+1; i++ {
			matched := true
			for j := 0; j < lm; j++ {
				if !okStr(words[i+j], match[j]) {
					matched = false
					break
				}
			}
			if matched {
				return i
			}
		}
	}
	return -1
}

func okStr(word string, matches []string) bool {
	for _, m := range matches {
		if word == m {
			return true
		}
	}
	return false
}
