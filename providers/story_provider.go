package providers

import (
	"context"
	"fmt"

	"github.com/PiotrWarzachowski/go-instagram-cli/internal/platform/instagram"
	"github.com/PiotrWarzachowski/go-instagram-cli/internal/storage"
)

type StoryProvider struct {
	ig *instagram.Client
}

func (p *StoryProvider) UploadWithProgress(ctx context.Context, videoPath string, reporter instagram.ProgressReporter) (*instagram.StoryPostResult, error) {
	if videoPath == "" {
		return nil, fmt.Errorf("video path cannot be empty")
	}

	result, err := p.ig.UploadStory(ctx, videoPath, reporter)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (p *StoryProvider) GetMyStories(ctx context.Context) (*instagram.StorySummary, error) {
	result, err := p.ig.GetMyStories(ctx)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func NewStoryProvider() (*StoryProvider, error) {
	storage, err := storage.NewSessionStorage()
	if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
	}

	session, err := storage.LoadSession()
	if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
	}

	igClient, err := instagram.NewClientFromSession(session)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}
	return &StoryProvider{
		ig: igClient,
	}, nil
}
