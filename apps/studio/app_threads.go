package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/rudrankriyam/App-Store-Connect-CLI/apps/studio/internal/studio/acp"
	"github.com/rudrankriyam/App-Store-Connect-CLI/apps/studio/internal/studio/approvals"
	"github.com/rudrankriyam/App-Store-Connect-CLI/apps/studio/internal/studio/settings"
	"github.com/rudrankriyam/App-Store-Connect-CLI/apps/studio/internal/studio/threads"
)

func (a *App) ListThreads() ([]StudioThread, error) {
	all, err := a.threads.LoadAll()
	if err != nil {
		return nil, err
	}
	return toStudioThreads(all), nil
}

func (a *App) CreateThread(title string) (StudioThread, error) {
	thread, err := a.createThreadRecord(title)
	if err != nil {
		return StudioThread{}, err
	}
	return toStudioThread(thread), nil
}

func (a *App) createThreadRecord(title string) (threads.Thread, error) {
	if strings.TrimSpace(title) == "" {
		title = "New Studio Thread"
	}

	now := time.Now().UTC()
	thread := threads.Thread{
		ID:        uuid.NewString(),
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := a.threads.SaveThread(thread); err != nil {
		return threads.Thread{}, err
	}
	return thread, nil
}

func (a *App) ResolveASC() (ResolutionResponse, error) {
	resolution, err := a.resolveASC()
	if err != nil {
		return ResolutionResponse{}, err
	}

	return ResolutionResponse{
		Resolution:       resolution,
		AvailablePresets: settings.DefaultPresets(),
	}, nil
}

func (a *App) QueueMutation(req ApprovalRequest) (StudioApproval, error) {
	if strings.TrimSpace(req.ThreadID) == "" {
		return StudioApproval{}, errors.New("thread ID is required")
	}
	if strings.TrimSpace(req.Title) == "" {
		return StudioApproval{}, errors.New("title is required")
	}

	action := approvals.Action{
		ID:              uuid.NewString(),
		ThreadID:        req.ThreadID,
		Title:           req.Title,
		Summary:         req.Summary,
		CommandPreview:  req.CommandPreview,
		MutationSurface: req.MutationSurface,
		Status:          approvals.StatusPending,
		CreatedAt:       time.Now().UTC(),
	}
	return toStudioApproval(a.approvals.Enqueue(action)), nil
}

func (a *App) ListApprovals() []StudioApproval {
	return toStudioApprovals(a.approvals.Pending())
}

func (a *App) ApproveAction(id string) (StudioApproval, error) {
	action, err := a.approvals.Approve(id)
	if err != nil {
		return StudioApproval{}, err
	}
	return toStudioApproval(action), nil
}

func (a *App) RejectAction(id string) (StudioApproval, error) {
	action, err := a.approvals.Reject(id)
	if err != nil {
		return StudioApproval{}, err
	}
	return toStudioApproval(action), nil
}

func (a *App) SendPrompt(req PromptRequest) (PromptResponse, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return PromptResponse{}, errors.New("prompt is required")
	}

	thread, err := a.ensureThread(req.ThreadID)
	if err != nil {
		return PromptResponse{}, err
	}

	thread.Messages = append(thread.Messages, threads.Message{
		ID:        uuid.NewString(),
		Role:      threads.RoleUser,
		Kind:      threads.KindMessage,
		Content:   req.Prompt,
		CreatedAt: time.Now().UTC(),
	})
	thread.UpdatedAt = time.Now().UTC()
	if err := a.threads.SaveThread(thread); err != nil {
		return PromptResponse{}, err
	}

	session, err := a.ensureSession(thread)
	if err != nil {
		return PromptResponse{}, err
	}

	ctx, cancel := context.WithTimeout(a.contextOrBackground(), 15*time.Second)
	defer cancel()

	result, events, err := session.client.Prompt(ctx, session.sessionID, req.Prompt)
	if err != nil {
		return PromptResponse{}, err
	}

	for _, event := range events {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "studio:agent:update", event)
		}
	}

	assistantMessage := result.Summary()
	if assistantMessage == "" {
		assistantMessage = "ASC Studio captured the prompt and is waiting for the agent response stream."
	}

	thread.Messages = append(thread.Messages, threads.Message{
		ID:        uuid.NewString(),
		Role:      threads.RoleAssistant,
		Kind:      threads.KindMessage,
		Content:   assistantMessage,
		CreatedAt: time.Now().UTC(),
	})
	thread.SessionID = session.sessionID
	thread.UpdatedAt = time.Now().UTC()
	if err := a.threads.SaveThread(thread); err != nil {
		return PromptResponse{}, err
	}

	return PromptResponse{Thread: toStudioThread(thread)}, nil
}

func (a *App) ensureThread(id string) (threads.Thread, error) {
	if strings.TrimSpace(id) == "" {
		return a.createThreadRecord("New Studio Thread")
	}
	return a.threads.Get(id)
}

func (a *App) ensureSession(thread threads.Thread) (*threadSession, error) {
	for {
		a.mu.Lock()
		if existing := a.sessions[thread.ID]; existing != nil {
			a.mu.Unlock()
			return existing, nil
		}
		if init, ok := a.sessionInits[thread.ID]; ok {
			if init.err != nil || init.panicValue != nil {
				panicValue := init.panicValue
				err := init.err
				if init.waiters == 0 {
					delete(a.sessionInits, thread.ID)
				}
				a.mu.Unlock()
				if panicValue != nil {
					panic(panicValue)
				}
				return nil, err
			}

			init.waiters++
			done := init.done
			a.mu.Unlock()
			<-done

			a.mu.Lock()
			current := a.sessionInits[thread.ID]
			if current == init {
				panicValue := init.panicValue
				err := init.err
				if init.waiters > 0 {
					init.waiters--
				}
				if init.waiters == 0 {
					delete(a.sessionInits, thread.ID)
				}
				a.mu.Unlock()
				if panicValue != nil {
					panic(panicValue)
				}
				if err != nil {
					return nil, err
				}
			} else {
				a.mu.Unlock()
			}
			continue
		}

		init := &sessionInit{done: make(chan struct{})}
		a.sessionInits[thread.ID] = init
		a.mu.Unlock()

		var (
			session    *threadSession
			err        error
			panicValue any
		)
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					panicValue = recovered
				}
			}()
			session, err = a.startThreadSession(thread)
		}()

		a.mu.Lock()
		if current := a.sessionInits[thread.ID]; current == init {
			if err == nil && panicValue == nil && session != nil {
				a.sessions[thread.ID] = session
				delete(a.sessionInits, thread.ID)
			} else {
				init.err = err
				init.panicValue = panicValue
				if init.waiters == 0 {
					delete(a.sessionInits, thread.ID)
				}
			}
			close(init.done)
		}
		a.mu.Unlock()

		if panicValue != nil {
			panic(panicValue)
		}
		if err != nil {
			return nil, err
		}
		return session, nil
	}
}

func (a *App) startThreadSession(thread threads.Thread) (*threadSession, error) {
	cfg, err := a.settings.Load()
	if err != nil {
		return nil, err
	}
	launch, err := cfg.ResolveAgentLaunch()
	if err != nil {
		return nil, err
	}

	client, err := a.startAgent(a.contextOrBackground(), acp.LaunchSpec{
		Command: launch.Command,
		Args:    launch.Args,
		Dir:     launch.Dir,
		Env:     launch.Env,
	})
	if err != nil {
		return nil, err
	}

	workspaceRoot := cfg.WorkspaceRoot
	if strings.TrimSpace(workspaceRoot) == "" {
		workspaceRoot, _ = getwdFunc()
	}

	bootstrapCtx, cancel := context.WithTimeout(a.contextOrBackground(), sessionInitTimeout)
	defer cancel()

	sessionID, err := client.Bootstrap(bootstrapCtx, acp.SessionConfig{
		CWD:       workspaceRoot,
		SessionID: thread.SessionID,
	})
	if err != nil {
		_ = client.Close()
		return nil, err
	}

	return &threadSession{
		client:    client,
		sessionID: sessionID,
	}, nil
}

func toStudioThreads(items []threads.Thread) []StudioThread {
	out := make([]StudioThread, 0, len(items))
	for _, item := range items {
		out = append(out, toStudioThread(item))
	}
	return out
}

func toStudioThread(item threads.Thread) StudioThread {
	messages := make([]StudioMessage, 0, len(item.Messages))
	for _, message := range item.Messages {
		messages = append(messages, StudioMessage{
			ID:        message.ID,
			Role:      message.Role,
			Kind:      message.Kind,
			Content:   message.Content,
			CreatedAt: formatTimestamp(message.CreatedAt),
		})
	}

	return StudioThread{
		ID:        item.ID,
		Title:     item.Title,
		SessionID: item.SessionID,
		CreatedAt: formatTimestamp(item.CreatedAt),
		UpdatedAt: formatTimestamp(item.UpdatedAt),
		Messages:  messages,
	}
}

func toStudioApprovals(items []approvals.Action) []StudioApproval {
	out := make([]StudioApproval, 0, len(items))
	for _, item := range items {
		out = append(out, toStudioApproval(item))
	}
	return out
}

func toStudioApproval(item approvals.Action) StudioApproval {
	return StudioApproval{
		ID:              item.ID,
		ThreadID:        item.ThreadID,
		Title:           item.Title,
		Summary:         item.Summary,
		CommandPreview:  item.CommandPreview,
		MutationSurface: item.MutationSurface,
		Status:          item.Status,
		CreatedAt:       formatTimestamp(item.CreatedAt),
		ResolvedAt:      formatTimestamp(item.ResolvedAt),
	}
}

func formatTimestamp(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.UTC().Format(time.RFC3339)
}
