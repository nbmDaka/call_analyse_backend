package worker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"call_analyse_backend/internal/modules/analysis"
	"call_analyse_backend/internal/modules/calls"
	"call_analyse_backend/internal/modules/scoring"
	"call_analyse_backend/internal/modules/transcription"
	"call_analyse_backend/internal/platform/queue"

	"github.com/google/uuid"
)

func TestProcessorProcessesCallOnceThroughCompleted(t *testing.T) {
	t.Parallel()

	call := testCall(calls.StatusUploaded)
	callStore := &memoryCallStore{call: call, history: []calls.Status{call.Status}}
	transcriber := &fakeTranscriber{result: testTranscript()}
	analyzer := &fakeAnalyzer{result: testAnalysis()}
	transcriptStore := &memoryTranscriptStore{}
	analysisStore := &memoryAnalysisStore{}
	processor := Processor{
		Calls:           callStore,
		Transcripts:     transcriptStore,
		Analyses:        analysisStore,
		Transcriber:     transcriber,
		Analyzer:        analyzer,
		Objects:         memoryObjectStore{data: []byte("audio-bytes")},
		ProviderTimeout: time.Second,
	}

	if err := processor.Process(context.Background(), call.ID.String()); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if err := processor.Process(context.Background(), call.ID.String()); err != nil {
		t.Fatalf("Process() duplicate delivery error = %v", err)
	}

	wantHistory := []calls.Status{
		calls.StatusUploaded,
		calls.StatusTranscribing,
		calls.StatusTranscribed,
		calls.StatusAnalyzing,
		calls.StatusCompleted,
	}
	if !reflect.DeepEqual(callStore.history, wantHistory) {
		t.Errorf("status history = %v, want %v", callStore.history, wantHistory)
	}
	if transcriber.calls != 1 {
		t.Errorf("transcriber calls = %d, want 1", transcriber.calls)
	}
	if analyzer.calls != 1 {
		t.Errorf("analyzer calls = %d, want 1", analyzer.calls)
	}
	if transcriptStore.upserts != 1 {
		t.Errorf("transcript upserts = %d, want 1", transcriptStore.upserts)
	}
	if analysisStore.upserts != 1 {
		t.Errorf("analysis and score upserts = %d, want 1", analysisStore.upserts)
	}
	if analysisStore.score.Total != 100 {
		t.Errorf("stored backend score total = %d, want 100", analysisStore.score.Total)
	}
}

func TestProcessorRequiresMatchingWorkspaceForTenantTask(t *testing.T) {
	call := testCall(calls.StatusCompleted)
	call.WorkspaceID = uuid.New()
	callStore := &memoryCallStore{call: call}
	processor := Processor{Calls: callStore, Transcripts: &memoryTranscriptStore{}, Analyses: &memoryAnalysisStore{}, Transcriber: &fakeTranscriber{}, Analyzer: &fakeAnalyzer{}, Objects: memoryObjectStore{}, ProviderTimeout: time.Second}
	if err := processor.ProcessInWorkspace(context.Background(), uuid.NewString(), call.ID.String()); err == nil {
		t.Fatal("ProcessInWorkspace() accepted a call from another workspace")
	}
	if err := processor.ProcessInWorkspace(context.Background(), call.WorkspaceID.String(), call.ID.String()); err != nil {
		t.Fatalf("ProcessInWorkspace() matching tenant error = %v", err)
	}
}

func TestProcessorSkipsTranscriptionForPersistedTranscript(t *testing.T) {
	t.Parallel()

	call := testCall(calls.StatusUploaded)
	callStore := &memoryCallStore{call: call, history: []calls.Status{call.Status}}
	transcriptStore := &memoryTranscriptStore{transcript: testTranscript(), exists: true}
	transcriber := &fakeTranscriber{result: testTranscript()}
	analyzer := &fakeAnalyzer{result: testAnalysis()}
	analysisStore := &memoryAnalysisStore{}
	processor := Processor{
		Calls:           callStore,
		Transcripts:     transcriptStore,
		Analyses:        analysisStore,
		Transcriber:     transcriber,
		Analyzer:        analyzer,
		Objects:         memoryObjectStore{data: []byte("audio-bytes")},
		ProviderTimeout: time.Second,
	}

	if err := processor.Process(context.Background(), call.ID.String()); err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if transcriber.calls != 0 {
		t.Errorf("transcriber calls = %d, want 0 when a transcript exists", transcriber.calls)
	}
	if transcriptStore.upserts != 0 {
		t.Errorf("transcript upserts = %d, want 0 when a transcript exists", transcriptStore.upserts)
	}
	if analyzer.calls != 1 {
		t.Errorf("analyzer calls = %d, want 1", analyzer.calls)
	}
}

func TestProcessorRetriesFromFailedWithoutRetranscribingPersistedTranscript(t *testing.T) {
	t.Parallel()

	call := testCall(calls.StatusFailed)
	callStore := &memoryCallStore{call: call, history: []calls.Status{call.Status}}
	transcriptStore := &memoryTranscriptStore{transcript: testTranscript(), exists: true}
	transcriber := &fakeTranscriber{result: testTranscript()}
	analyzer := &fakeAnalyzer{result: testAnalysis()}
	analysisStore := &memoryAnalysisStore{}
	processor := Processor{
		Calls:           callStore,
		Transcripts:     transcriptStore,
		Analyses:        analysisStore,
		Transcriber:     transcriber,
		Analyzer:        analyzer,
		Objects:         memoryObjectStore{data: []byte("audio-bytes")},
		ProviderTimeout: time.Second,
	}

	if err := processor.Process(context.Background(), call.ID.String()); err != nil {
		t.Fatalf("Process() retry error = %v", err)
	}

	wantHistory := []calls.Status{
		calls.StatusFailed,
		calls.StatusQueued,
		calls.StatusTranscribing,
		calls.StatusTranscribed,
		calls.StatusAnalyzing,
		calls.StatusCompleted,
	}
	if !reflect.DeepEqual(callStore.history, wantHistory) {
		t.Errorf("retry status history = %v, want %v", callStore.history, wantHistory)
	}
	if transcriber.calls != 0 {
		t.Errorf("transcriber calls = %d, want 0 when retrying a persisted transcript", transcriber.calls)
	}
}

func TestProcessorMarksProviderFailureWithSanitizedError(t *testing.T) {
	t.Parallel()

	call := testCall(calls.StatusUploaded)
	callStore := &memoryCallStore{call: call, history: []calls.Status{call.Status}}
	providerError := errors.New("status 403: api_key=secret-value provider response body")
	processor := Processor{
		Calls:           callStore,
		Transcripts:     &memoryTranscriptStore{},
		Analyses:        &memoryAnalysisStore{},
		Transcriber:     &fakeTranscriber{err: providerError},
		Analyzer:        &fakeAnalyzer{result: testAnalysis()},
		Objects:         memoryObjectStore{data: []byte("audio-bytes")},
		ProviderTimeout: time.Second,
	}

	err := processor.Process(context.Background(), call.ID.String())
	if err == nil {
		t.Fatal("Process() error = nil, want provider failure")
	}
	if got, want := err.Error(), "transcription provider failed"; got != want {
		t.Errorf("Process() error = %q, want %q", got, want)
	}
	if callStore.call.Status != calls.StatusFailed {
		t.Errorf("call status = %q, want %q", callStore.call.Status, calls.StatusFailed)
	}
	if callStore.call.ErrorMessage == nil {
		t.Fatal("call error message = nil, want sanitized provider error")
	}
	if got, want := *callStore.call.ErrorMessage, "transcription provider failed"; got != want {
		t.Errorf("stored error = %q, want %q", got, want)
	}
	if strings.Contains(*callStore.call.ErrorMessage, "secret-value") {
		t.Errorf("stored error exposes provider content: %q", *callStore.call.ErrorMessage)
	}
}

func TestProcessorSerializesOverlappingDeliveryForOneCall(t *testing.T) {
	call := testCall(calls.StatusTranscribing)
	callStore := &memoryCallStore{call: call, history: []calls.Status{call.Status}}
	transcriber := &blockingTranscriber{
		result:  testTranscript(),
		entered: make(chan struct{}, 2),
		release: make(chan struct{}),
	}
	processor := Processor{
		Calls:           callStore,
		Transcripts:     &memoryTranscriptStore{},
		Analyses:        &memoryAnalysisStore{},
		Transcriber:     transcriber,
		Analyzer:        &fakeAnalyzer{result: testAnalysis()},
		Objects:         memoryObjectStore{data: []byte("audio-bytes")},
		ProviderTimeout: time.Second,
	}

	errors := make(chan error, 2)
	go func() { errors <- processor.Process(context.Background(), call.ID.String()) }()
	select {
	case <-transcriber.entered:
	case <-time.After(time.Second):
		t.Fatal("first Process() call did not reach the blocking transcription provider")
	}

	go func() { errors <- processor.Process(context.Background(), call.ID.String()) }()
	select {
	case <-transcriber.entered:
		close(transcriber.release)
		firstError := <-errors
		secondError := <-errors
		t.Fatalf("overlapping Process() calls reached transcription twice: errors = %v, %v", firstError, secondError)
	case <-time.After(100 * time.Millisecond):
		close(transcriber.release)
	}

	for range 2 {
		if err := <-errors; err != nil {
			t.Fatalf("Process() error = %v", err)
		}
	}
	if got, want := transcriber.CallCount(), 1; got != want {
		t.Errorf("transcription provider calls = %d, want %d", got, want)
	}
}

func TestProcessorAllowsUnrelatedCallsToProcessConcurrently(t *testing.T) {
	firstCall := testCall(calls.StatusTranscribing)
	secondCall := testCall(calls.StatusTranscribing)
	secondCall.ID = uuid.MustParse("00000000-0000-0000-0000-000000000702")
	transcriber := &blockingTranscriber{
		result:  testTranscript(),
		entered: make(chan struct{}, 2),
		release: make(chan struct{}),
	}
	processor := Processor{
		Calls: &multiCallStore{calls: map[uuid.UUID]calls.Call{
			firstCall.ID:  firstCall,
			secondCall.ID: secondCall,
		}},
		Transcripts:     &memoryTranscriptStore{},
		Analyses:        &memoryAnalysisStore{},
		Transcriber:     transcriber,
		Analyzer:        &fakeAnalyzer{result: testAnalysis()},
		Objects:         memoryObjectStore{data: []byte("audio-bytes")},
		ProviderTimeout: time.Second,
	}

	firstCtx, cancelFirst := context.WithCancel(context.Background())
	secondCtx, cancelSecond := context.WithCancel(context.Background())
	defer cancelFirst()
	defer cancelSecond()
	errors := make(chan error, 2)
	go func() { errors <- processor.Process(firstCtx, firstCall.ID.String()) }()
	select {
	case <-transcriber.entered:
	case <-time.After(time.Second):
		t.Fatal("first call did not reach the blocking transcription provider")
	}

	go func() { errors <- processor.Process(secondCtx, secondCall.ID.String()) }()
	select {
	case <-transcriber.entered:
	case <-time.After(100 * time.Millisecond):
		cancelFirst()
		cancelSecond()
		<-errors
		<-errors
		t.Fatal("unrelated calls were serialized before transcription")
	}

	cancelFirst()
	cancelSecond()
	<-errors
	<-errors
	if got, want := transcriber.CallCount(), 2; got != want {
		t.Errorf("transcription provider calls = %d, want %d for unrelated calls", got, want)
	}
}

func TestHandlerReturnsProcessorErrorForAsynqRetry(t *testing.T) {
	t.Parallel()

	call := testCall(calls.StatusUploaded)
	processor := &Processor{
		Calls:           &memoryCallStore{call: call, history: []calls.Status{call.Status}},
		Transcripts:     &memoryTranscriptStore{},
		Analyses:        &memoryAnalysisStore{},
		Transcriber:     &fakeTranscriber{err: errors.New("provider failed")},
		Analyzer:        &fakeAnalyzer{result: testAnalysis()},
		Objects:         memoryObjectStore{data: []byte("audio-bytes")},
		ProviderTimeout: time.Second,
	}
	task, err := queue.NewProcessCallTask(call.ID.String())
	if err != nil {
		t.Fatalf("NewProcessCallTask() error = %v", err)
	}

	err = NewHandler(processor).ProcessTask(context.Background(), task)
	if err == nil {
		t.Fatal("ProcessTask() error = nil, want an error for Asynq retry")
	}
	if got, want := err.Error(), "transcription provider failed"; got != want {
		t.Errorf("ProcessTask() error = %q, want %q", got, want)
	}
}

func testCall(status calls.Status) calls.Call {
	return calls.Call{
		ID:          uuid.MustParse("00000000-0000-0000-0000-000000000701"),
		Status:      status,
		ObjectKey:   "calls/701/audio.mp3",
		ContentType: "audio/mpeg",
	}
}

func testTranscript() transcription.Transcript {
	return transcription.Transcript{
		Text:     "manager: Hello\nclient: Please send pricing.",
		Segments: []transcription.Segment{{Speaker: transcription.SpeakerManager, Text: "Hello"}},
	}
}

func testAnalysis() analysis.Analysis {
	criteria := make(map[string]analysis.CriterionResult, len(scoring.Criteria()))
	for _, criterion := range scoring.Criteria() {
		criteria[criterion.Key] = analysis.CriterionResult{Score: criterion.Max, Feedback: "Specific coaching feedback."}
	}
	return analysis.Analysis{
		Summary:          "The client requested pricing.",
		Needs:            []string{"Pricing"},
		Objections:       []string{},
		Mistakes:         []string{},
		Strengths:        []string{"Clear greeting"},
		NextAction:       "Send pricing.",
		CriterionResults: criteria,
	}
}

type memoryCallStore struct {
	call    calls.Call
	history []calls.Status
}

func (s *memoryCallStore) Get(ctx context.Context, callID uuid.UUID) (calls.Call, error) {
	if err := ctx.Err(); err != nil {
		return calls.Call{}, err
	}
	if callID != s.call.ID {
		return calls.Call{}, fmt.Errorf("call not found")
	}
	return s.call, nil
}

func (s *memoryCallStore) GetInWorkspace(ctx context.Context, workspaceID, callID uuid.UUID) (calls.Call, error) {
	call, err := s.Get(ctx, callID)
	if err != nil || call.WorkspaceID != workspaceID {
		return calls.Call{}, fmt.Errorf("call not found")
	}
	return call, nil
}

func (s *memoryCallStore) Transition(ctx context.Context, callID uuid.UUID, from, to calls.Status, errorMessage *string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if callID != s.call.ID {
		return fmt.Errorf("call not found")
	}
	if s.call.Status != from {
		return fmt.Errorf("call status = %q, want %q", s.call.Status, from)
	}
	if err := calls.ValidateTransition(from, to); err != nil {
		return err
	}
	s.call.Status = to
	s.call.ErrorMessage = errorMessage
	s.history = append(s.history, to)
	return nil
}

func (s *memoryCallStore) TransitionInWorkspace(ctx context.Context, workspaceID, callID uuid.UUID, from, to calls.Status, errorMessage *string) error {
	if workspaceID != s.call.WorkspaceID {
		return fmt.Errorf("call not found")
	}
	return s.Transition(ctx, callID, from, to, errorMessage)
}

type multiCallStore struct {
	mu    sync.Mutex
	calls map[uuid.UUID]calls.Call
}

func (s *multiCallStore) Get(ctx context.Context, callID uuid.UUID) (calls.Call, error) {
	if err := ctx.Err(); err != nil {
		return calls.Call{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	call, ok := s.calls[callID]
	if !ok {
		return calls.Call{}, fmt.Errorf("call not found")
	}
	return call, nil
}

func (s *multiCallStore) Transition(ctx context.Context, callID uuid.UUID, from, to calls.Status, errorMessage *string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	call, ok := s.calls[callID]
	if !ok {
		return fmt.Errorf("call not found")
	}
	if call.Status != from {
		return fmt.Errorf("call status = %q, want %q", call.Status, from)
	}
	if err := calls.ValidateTransition(from, to); err != nil {
		return err
	}
	call.Status = to
	call.ErrorMessage = errorMessage
	s.calls[callID] = call
	return nil
}

type memoryTranscriptStore struct {
	transcript transcription.Transcript
	exists     bool
	upserts    int
}

func (s *memoryTranscriptStore) Get(ctx context.Context, callID uuid.UUID) (transcription.Transcript, bool, error) {
	if err := ctx.Err(); err != nil {
		return transcription.Transcript{}, false, err
	}
	return s.transcript, s.exists, nil
}

func (s *memoryTranscriptStore) Upsert(ctx context.Context, callID uuid.UUID, transcript transcription.Transcript) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.transcript = transcript
	s.exists = true
	s.upserts++
	return nil
}

type memoryAnalysisStore struct {
	exists   bool
	analysis analysis.Analysis
	score    scoring.Score
	upserts  int
}

func (s *memoryAnalysisStore) Exists(ctx context.Context, callID uuid.UUID) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	return s.exists, nil
}

func (s *memoryAnalysisStore) UpsertWithScore(ctx context.Context, callID uuid.UUID, result analysis.Analysis, score scoring.Score) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.exists = true
	s.analysis = result
	s.score = score
	s.upserts++
	return nil
}

type memoryObjectStore struct {
	data []byte
}

func (s memoryObjectStore) Put(context.Context, string, io.Reader, int64, string) error {
	return nil
}

func (s memoryObjectStore) Get(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.data)), nil
}

func (s memoryObjectStore) Delete(context.Context, string) error {
	return nil
}

type fakeTranscriber struct {
	result transcription.TranscriptResult
	err    error
	calls  int
}

func (p *fakeTranscriber) Transcribe(ctx context.Context, input transcription.AudioInput) (transcription.TranscriptResult, error) {
	if err := ctx.Err(); err != nil {
		return transcription.TranscriptResult{}, err
	}
	p.calls++
	return p.result, p.err
}

type fakeAnalyzer struct {
	result analysis.Analysis
	err    error
	calls  int
}

func (p *fakeAnalyzer) Analyze(ctx context.Context, transcript transcription.Transcript) (analysis.Analysis, error) {
	if err := ctx.Err(); err != nil {
		return analysis.Analysis{}, err
	}
	p.calls++
	return p.result, p.err
}

type blockingTranscriber struct {
	mu      sync.Mutex
	result  transcription.TranscriptResult
	entered chan struct{}
	release chan struct{}
	calls   int
}

func (p *blockingTranscriber) Transcribe(ctx context.Context, input transcription.AudioInput) (transcription.TranscriptResult, error) {
	p.mu.Lock()
	p.calls++
	p.mu.Unlock()
	p.entered <- struct{}{}
	select {
	case <-p.release:
		return p.result, nil
	case <-ctx.Done():
		return transcription.TranscriptResult{}, ctx.Err()
	}
}

func (p *blockingTranscriber) CallCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}
