package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/PaulRychkov/task-planner/backend/internal/models"
	"github.com/PaulRychkov/task-planner/backend/internal/repository"
)

type TopicInput struct {
	Name        string     `json:"name"`
	ParentID    *uuid.UUID `json:"parent_id"`
	Description *string    `json:"description"`
	IsArchived  *bool      `json:"is_archived"`
}

type TopicService struct {
	store repository.Store
}

func NewTopicService(store repository.Store) *TopicService {
	return &TopicService{store: store}
}

func (s *TopicService) List(ctx context.Context, includeArchived bool) ([]models.Topic, error) {
	return s.store.Topics().List(ctx, includeArchived)
}

func (s *TopicService) Get(ctx context.Context, id uuid.UUID) (*models.Topic, error) {
	return s.store.Topics().Get(ctx, id)
}

func (s *TopicService) Create(ctx context.Context, in TopicInput) (*models.Topic, error) {
	if in.Name == "" {
		return nil, invalid("name is required")
	}
	topic := &models.Topic{Name: in.Name, ParentID: in.ParentID, Description: in.Description}
	if in.IsArchived != nil {
		topic.IsArchived = *in.IsArchived
	}
	if in.ParentID != nil {
		if _, err := s.store.Topics().Get(ctx, *in.ParentID); err != nil {
			if IsNotFound(err) {
				return nil, invalid("parent topic not found")
			}
			return nil, err
		}
	}
	if err := s.store.Topics().Create(ctx, topic); err != nil {
		if IsConflict(err) {
			return nil, conflict("topic with this name already exists")
		}
		return nil, err
	}
	return topic, nil
}

func (s *TopicService) Update(ctx context.Context, id uuid.UUID, in TopicInput) (*models.Topic, error) {
	if in.Name == "" {
		return nil, invalid("name is required")
	}
	topic, err := s.store.Topics().Get(ctx, id)
	if err != nil {
		return nil, err
	}
	topic.Name = in.Name
	topic.Description = in.Description
	if err := s.validateParent(ctx, id, in.ParentID); err != nil {
		return nil, err
	}
	topic.ParentID = in.ParentID
	if in.IsArchived != nil {
		topic.IsArchived = *in.IsArchived
	}
	if err := s.store.Topics().Update(ctx, topic); err != nil {
		if IsConflict(err) {
			return nil, conflict("topic with this name already exists")
		}
		return nil, err
	}
	return topic, nil
}

func (s *TopicService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.store.Topics().Delete(ctx, id)
}

func (s *TopicService) FindOrCreate(ctx context.Context, name string) (*models.Topic, error) {
	return s.FindOrCreateUnder(ctx, name, nil)
}

func (s *TopicService) FindOrCreateUnder(ctx context.Context, name string, parentID *uuid.UUID) (*models.Topic, error) {
	if name == "" {
		return nil, invalid("topic name is required")
	}
	topic, err := s.store.Topics().GetByName(ctx, name)
	if err == nil {
		if parentID != nil && topic.ParentID == nil {
			if verr := s.validateParent(ctx, topic.ID, parentID); verr != nil {
				return nil, verr
			}
			topic.ParentID = parentID
			if uerr := s.store.Topics().Update(ctx, topic); uerr != nil {
				return nil, uerr
			}
		}
		return topic, nil
	}
	if !IsNotFound(err) {
		return nil, err
	}
	return s.Create(ctx, TopicInput{Name: name, ParentID: parentID})
}

func (s *TopicService) validateParent(ctx context.Context, id uuid.UUID, parentID *uuid.UUID) error {
	if parentID == nil {
		return nil
	}
	seen := map[uuid.UUID]bool{id: true}
	current := parentID
	for current != nil {
		if seen[*current] {
			return invalid("topic parent would create a cycle")
		}
		seen[*current] = true
		parent, err := s.store.Topics().Get(ctx, *current)
		if err != nil {
			if IsNotFound(err) {
				if *current == *parentID {
					return invalid("parent topic not found")
				}
				return nil
			}
			return err
		}
		current = parent.ParentID
	}
	return nil
}
