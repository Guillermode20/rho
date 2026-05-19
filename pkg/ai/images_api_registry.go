package ai

import (
	"fmt"
	"sync"
)

// ImagesAPIRegistry manages image generation provider registrations.
type ImagesAPIRegistry struct {
	mu       sync.RWMutex
	registry map[ImagesApi]ImagesFunction
}

var (
	imageRegistry   = &ImagesAPIRegistry{registry: make(map[ImagesApi]ImagesFunction)}
	imageRegistryMu sync.RWMutex
)

// RegisterImagesProvider registers an image generation function for an API.
func RegisterImagesProvider(api ImagesApi, fn ImagesFunction) {
	imageRegistryMu.Lock()
	imageRegistry.registry[api] = fn
	imageRegistryMu.Unlock()
}

// GetImagesProvider returns the image generation function for an API.
func GetImagesProvider(api ImagesApi) (ImagesFunction, bool) {
	imageRegistryMu.RLock()
	defer imageRegistryMu.RUnlock()
	fn, ok := imageRegistry.registry[api]
	return fn, ok
}

// GenerateImage generates images using the specified model.
func GenerateImage(model ImageModel, ctx ImagesContext, options *ImagesOptions) (*AssistantImages, error) {
	fn, ok := GetImagesProvider(model.API)
	if !ok {
		return nil, fmt.Errorf("no image provider registered for api %q", model.API)
	}
	return fn(model, ctx, options)
}

// DefaultImageModels returns the built-in image generation models.
func DefaultImageModels() []ImageModel {
	return []ImageModel{
		{
			API:         ImagesAPIOpenRouter,
			Provider:    ImagesProviderOpenRouter,
			Name:        "dall-e-3",
			Input:       []string{"text"},
			Cost:        CostPerMillion{Input: 0.040, Output: 0.120},
			Description: "OpenAI DALL-E 3, 1024x1024, standard quality",
		},
		{
			API:         ImagesAPIOpenRouter,
			Provider:    ImagesProviderOpenRouter,
			Name:        "dall-e-2",
			Input:       []string{"text"},
			Cost:        CostPerMillion{Input: 0.020, Output: 0.020},
			Description: "OpenAI DALL-E 2, 1024x1024",
		},
		{
			API:         ImagesAPIOpenRouter,
			Provider:    ImagesProviderOpenRouter,
			Name:        "stabilityai/stable-diffusion-xl",
			Input:       []string{"text"},
			Cost:        CostPerMillion{Input: 0.006, Output: 0.006},
			Description: "Stability AI SDXL",
		},
	}
}
