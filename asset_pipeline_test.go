package main

import (
	"context"
	"errors"
	"testing"
)

type fakeStorage struct {
	found bool
	ops   []string
}

func (f *fakeStorage) CreateBucket(context.Context, string) error { return nil }
func (f *fakeStorage) Presign(_ context.Context, _, _ string, req PresignRequest) (PresignResult, error) {
	f.ops = append(f.ops, req.Op)
	return PresignResult{URL: "https://signed.example/asset"}, nil
}
func (f *fakeStorage) Head(context.Context, string, string) (HeadResult, error) {
	return HeadResult{Found: f.found}, nil
}

func TestAssetHandoffDecision(t *testing.T) {
	tests := []struct {
		name      string
		found     bool
		wantState AssetState
		wantErr   error
	}{
		{name: "object is visible and processing is queued", found: true, wantState: StateProcessing},
		{name: "missing object remains awaiting upload", found: false, wantState: StateAwaitingUpload, wantErr: ErrObjectNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &fakeStorage{found: tt.found}
			pipeline := NewAssetPipeline(storage, "media")
			intent, err := pipeline.BeginUpload(context.Background(), "asset-42", "creator-7", "trailer.mp4", "video/mp4", 8<<20)
			if err != nil {
				t.Fatal(err)
			}
			asset, err := pipeline.ConfirmUpload(context.Background(), intent.Asset.ID)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ConfirmUpload error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				asset, _ = pipeline.get(intent.Asset.ID)
			}
			if asset.State != tt.wantState {
				t.Fatalf("state = %q, want %q", asset.State, tt.wantState)
			}
		})
	}
}

func TestReadyAssetGetsCreatorDelivery(t *testing.T) {
	storage := &fakeStorage{found: true}
	pipeline := NewAssetPipeline(storage, "media")
	intent, _ := pipeline.BeginUpload(context.Background(), "asset-9", "creator-2", "clip.mp4", "video/mp4", 1024)
	processing, err := pipeline.ConfirmUpload(context.Background(), intent.Asset.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pipeline.CompleteJob(processing.ID, processing.JobID); err != nil {
		t.Fatal(err)
	}
	delivery, err := pipeline.CreatorDelivery(context.Background(), processing.ID, "creator-2")
	if err != nil {
		t.Fatal(err)
	}
	if delivery.URL == "" || len(storage.ops) != 2 || storage.ops[1] != "get" {
		t.Fatalf("delivery = %#v, operations = %v", delivery, storage.ops)
	}
}
