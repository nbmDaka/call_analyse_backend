// Package worker coordinates durable call-processing jobs.
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"call_analyse_backend/internal/analysis"
	"call_analyse_backend/internal/calls"
	"call_analyse_backend/internal/scoring"
	"call_analyse_backend/internal/storage"
	"call_analyse_backend/internal/transcription"

	"github.com/google/uuid"
)

type (
	// CallProcessingStore is the processing subset of call persistence.
	CallProcessingStore = calls.CallProcessingStore
	// TranscriptStore persists the transcription checkpoint.
	TranscriptStore = transcription.TranscriptStore
	// AnalysisStore persists the analysis and calculated score together.
	AnalysisStore = analysis.AnalysisStore
)

const (
	transcriptionProviderFailure = "transcription provider failed"
	analysisProviderFailure      = "analysis provider failed"
	processingFailure            = "call processing failed"
)

// Processor executes one idempotent call-processing state machine.
type Processor struct {
	Calls           CallProcessingStore
	Transcripts     TranscriptStore
	Analyses        AnalysisStore
	Transcriber     transcription.TranscriptionProvider
	Analyzer        analysis.AnalysisProvider
	Objects         storage.ObjectStore
	ProviderTimeout time.Duration

	// callLocks serializes overlapping deliveries for one call within this
	// worker process. Database conditional transitions and upserts remain
	// necessary for duplicate deliveries handled by separate processes.
	callLocks callLockRegistry
}

type callLockRegistry struct {
	mu    sync.Mutex
	locks map[uuid.UUID]*callLock
}

type callLock struct {
	mu   sync.Mutex
	refs int
}

// Lock acquires the mutex for callID and returns a release function. The
// reference count includes callers waiting on the mutex so removing an idle
// entry cannot let a later caller bypass a waiting one.
func (r *callLockRegistry) Lock(callID uuid.UUID) func() {
	r.mu.Lock()
	if r.locks == nil {
		r.locks = make(map[uuid.UUID]*callLock)
	}
	lock := r.locks[callID]
	if lock == nil {
		lock = &callLock{}
		r.locks[callID] = lock
	}
	lock.refs++
	r.mu.Unlock()

	lock.mu.Lock()
	return func() {
		lock.mu.Unlock()

		r.mu.Lock()
		lock.refs--
		if lock.refs == 0 {
			delete(r.locks, callID)
		}
		r.mu.Unlock()
	}
}

// Process advances a call from its current durable checkpoint. It returns a
// sanitized error after marking failures so an Asynq handler can retry the job.
func (p *Processor) Process(ctx context.Context, callID string) error {
	if err := p.validate(); err != nil {
		return err
	}
	id, err := uuid.Parse(callID)
	if err != nil {
		return fmt.Errorf("parse call ID: %w", err)
	}
	release := p.callLocks.Lock(id)
	defer release()

	call, err := p.Calls.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("load call: %w", err)
	}

	for {
		switch call.Status {
		case calls.StatusCompleted:
			return nil
		case calls.StatusFailed:
			if err := p.transition(ctx, id, &call, calls.StatusQueued, nil); err != nil {
				return err
			}
		case calls.StatusUploaded:
			if err := p.transition(ctx, id, &call, calls.StatusTranscribing, nil); err != nil {
				return err
			}
		case calls.StatusQueued:
			if err := p.transition(ctx, id, &call, calls.StatusTranscribing, nil); err != nil {
				return err
			}
		case calls.StatusTranscribing:
			transcript, exists, err := p.Transcripts.Get(ctx, id)
			if err != nil {
				return p.fail(ctx, id, call, processingFailure)
			}
			if !exists {
				transcript, err = p.transcribe(ctx, call)
				if err != nil {
					return p.fail(ctx, id, call, transcriptionProviderFailure)
				}
				if err := p.Transcripts.Upsert(ctx, id, transcript); err != nil {
					return p.fail(ctx, id, call, processingFailure)
				}
			}
			if err := p.transition(ctx, id, &call, calls.StatusTranscribed, nil); err != nil {
				return err
			}
		case calls.StatusTranscribed:
			if err := p.transition(ctx, id, &call, calls.StatusAnalyzing, nil); err != nil {
				return err
			}
		case calls.StatusAnalyzing:
			exists, err := p.Analyses.Exists(ctx, id)
			if err != nil {
				return p.fail(ctx, id, call, processingFailure)
			}
			if !exists {
				transcript, transcriptExists, err := p.Transcripts.Get(ctx, id)
				if err != nil || !transcriptExists {
					return p.fail(ctx, id, call, processingFailure)
				}
				result, err := p.analyze(ctx, transcript)
				if err != nil {
					return p.fail(ctx, id, call, analysisProviderFailure)
				}
				validated, err := validateAnalysis(result)
				if err != nil {
					return p.fail(ctx, id, call, analysisProviderFailure)
				}
				score, err := scoring.Calculate(validated.CriterionResults)
				if err != nil {
					return p.fail(ctx, id, call, analysisProviderFailure)
				}
				if err := p.Analyses.UpsertWithScore(ctx, id, validated, score); err != nil {
					return p.fail(ctx, id, call, processingFailure)
				}
			}
			if err := p.transition(ctx, id, &call, calls.StatusCompleted, nil); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported call status %q", call.Status)
		}
	}
}

func (p *Processor) validate() error {
	if p.Calls == nil || p.Transcripts == nil || p.Analyses == nil || p.Transcriber == nil || p.Analyzer == nil || p.Objects == nil {
		return errors.New("worker processor dependencies are required")
	}
	if p.ProviderTimeout <= 0 {
		return errors.New("worker provider timeout must be greater than zero")
	}
	return nil
}

func (p *Processor) transition(ctx context.Context, callID uuid.UUID, call *calls.Call, to calls.Status, errorMessage *string) error {
	if err := p.Calls.Transition(ctx, callID, call.Status, to, errorMessage); err != nil {
		return fmt.Errorf("transition call to %q: %w", to, err)
	}
	call.Status = to
	call.ErrorMessage = errorMessage
	return nil
}

func (p *Processor) fail(ctx context.Context, callID uuid.UUID, call calls.Call, message string) error {
	if call.Status != calls.StatusFailed {
		if err := p.transition(ctx, callID, &call, calls.StatusFailed, &message); err != nil {
			return err
		}
	}
	return errors.New(message)
}

func (p *Processor) transcribe(ctx context.Context, call calls.Call) (transcription.Transcript, error) {
	reader, err := p.Objects.Get(ctx, call.ObjectKey)
	if err != nil {
		return transcription.Transcript{}, err
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		return transcription.Transcript{}, err
	}
	providerCtx, cancel := context.WithTimeout(ctx, p.ProviderTimeout)
	defer cancel()
	return p.Transcriber.Transcribe(providerCtx, transcription.AudioInput{
		Filename: call.OriginalFilename,
		MIMEType: call.ContentType,
		Data:     data,
	})
}

func (p *Processor) analyze(ctx context.Context, transcript transcription.Transcript) (analysis.Analysis, error) {
	providerCtx, cancel := context.WithTimeout(ctx, p.ProviderTimeout)
	defer cancel()
	return p.Analyzer.Analyze(providerCtx, transcript)
}

func validateAnalysis(result analysis.Analysis) (analysis.Analysis, error) {
	raw := result.RawJSON
	if len(raw) == 0 {
		encoded, err := json.Marshal(result)
		if err != nil {
			return analysis.Analysis{}, err
		}
		raw = encoded
	}
	return analysis.ParseAndValidate(raw)
}
