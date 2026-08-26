package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type AssetState string

const (
	StateAwaitingUpload AssetState = "awaiting_upload"
	StateProcessing     AssetState = "processing"
	StateReady          AssetState = "ready"
)

var (
	ErrAssetNotFound     = errors.New("asset not found")
	ErrObjectNotFound    = errors.New("uploaded object not found")
	ErrInvalidTransition = errors.New("invalid asset state transition")
)

type Asset struct {
	ID          string     `json:"id"`
	CreatorID   string     `json:"creator_id"`
	ObjectKey   string     `json:"object_key"`
	ContentType string     `json:"content_type"`
	State       AssetState `json:"state"`
	JobID       string     `json:"job_id,omitempty"`
}

type Storage interface {
	CreateBucket(context.Context, string) error
	Presign(context.Context, string, string, PresignRequest) (PresignResult, error)
	Head(context.Context, string, string) (HeadResult, error)
}

type AssetPipeline struct {
	storage Storage
	bucket  string
	mu      sync.RWMutex
	assets  map[string]Asset
}

func NewAssetPipeline(storage Storage, bucket string) *AssetPipeline {
	return &AssetPipeline{storage: storage, bucket: bucket, assets: make(map[string]Asset)}
}

type UploadIntent struct {
	Asset     Asset  `json:"asset"`
	UploadURL string `json:"upload_url"`
	Method    string `json:"method"`
}

func (p *AssetPipeline) BeginUpload(ctx context.Context, id, creatorID, filename, contentType string, maxBytes int64) (UploadIntent, error) {
	key := fmt.Sprintf("creators/%s/assets/%s/source/%s", creatorID, id, filename)
	asset := Asset{ID: id, CreatorID: creatorID, ObjectKey: key, ContentType: contentType, State: StateAwaitingUpload}
	signed, err := p.storage.Presign(ctx, p.bucket, key, PresignRequest{
		Op: "put", ExpiresSeconds: 600, ContentType: contentType, MaxBytes: maxBytes,
		IdempotencyKey: "asset-upload-" + id,
	})
	if err != nil {
		return UploadIntent{}, err
	}
	p.mu.Lock()
	p.assets[id] = asset
	p.mu.Unlock()
	return UploadIntent{Asset: asset, UploadURL: signed.URL, Method: "PUT"}, nil
}

func (p *AssetPipeline) ConfirmUpload(ctx context.Context, id string) (Asset, error) {
	asset, err := p.get(id)
	if err != nil {
		return Asset{}, err
	}
	if asset.State != StateAwaitingUpload {
		return Asset{}, ErrInvalidTransition
	}
	head, err := p.storage.Head(ctx, p.bucket, asset.ObjectKey)
	if err != nil {
		return Asset{}, err
	}
	if !head.Found {
		return Asset{}, ErrObjectNotFound
	}
	asset.State = StateProcessing
	asset.JobID = "process-" + asset.ID
	p.put(asset)
	return asset, nil
}

func (p *AssetPipeline) CompleteJob(id, jobID string) (Asset, error) {
	asset, err := p.get(id)
	if err != nil {
		return Asset{}, err
	}
	if asset.State != StateProcessing || asset.JobID != jobID {
		return Asset{}, ErrInvalidTransition
	}
	asset.State = StateReady
	p.put(asset)
	return asset, nil
}

func (p *AssetPipeline) CreatorDelivery(ctx context.Context, id, creatorID string) (PresignResult, error) {
	asset, err := p.get(id)
	if err != nil {
		return PresignResult{}, err
	}
	if asset.CreatorID != creatorID || asset.State != StateReady {
		return PresignResult{}, ErrInvalidTransition
	}
	return p.storage.Presign(ctx, p.bucket, asset.ObjectKey, PresignRequest{
		Op: "get", ExpiresSeconds: 300, IdempotencyKey: "asset-delivery-" + id,
	})
}

func (p *AssetPipeline) get(id string) (Asset, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	asset, ok := p.assets[id]
	if !ok {
		return Asset{}, ErrAssetNotFound
	}
	return asset, nil
}

func (p *AssetPipeline) put(asset Asset) {
	p.mu.Lock()
	p.assets[asset.ID] = asset
	p.mu.Unlock()
}
