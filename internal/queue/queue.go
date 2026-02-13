package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

const (
	// StreamTasks is the Redis stream for tasks (Memorizer pushes, Researcher pops).
	StreamTasks = "agent_tasks"
	// StreamProposals is the Redis stream for proposals (Researcher pushes, Memorizer pops).
	StreamProposals = "agent_proposals"

	// GroupResearcher is the consumer group for Researcher agents.
	GroupResearcher = "researcher_pool"
	// GroupMemorizer is the consumer group for Memorizer agents.
	GroupMemorizer = "memorizer_pool"
)

// TaskMessage is the payload pushed to the agent_tasks stream.
type TaskMessage struct {
	TurnID     string `json:"turn_id"`
	RegionPath string `json:"region_path"`
	ContextRef string `json:"context_ref,omitempty"`
	TaskType   string `json:"task_type"`
	Prompt     string `json:"prompt,omitempty"`
	Review     string `json:"review,omitempty"` // for review_response tasks
}

// ProposalMessage is the payload pushed to the agent_proposals stream.
type ProposalMessage struct {
	TurnID     string `json:"turn_id"`
	ProposalID string `json:"proposal_id"`
	RegionPath string `json:"region_path"`
}

// Queue manages Redis streams for inter-agent communication.
type Queue struct {
	client *redis.Client
}

// New creates a Queue from a Redis client.
func New(client *redis.Client) *Queue {
	return &Queue{client: client}
}

// ConnectRedis creates a Redis client from a URL.
func ConnectRedis(redisURL string) (*redis.Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis URL: %w", err)
	}
	return redis.NewClient(opts), nil
}

// EnsureStreams creates the consumer groups if they don't exist.
func (q *Queue) EnsureStreams(ctx context.Context) error {
	for _, pair := range []struct {
		stream, group string
	}{
		{StreamTasks, GroupResearcher},
		{StreamProposals, GroupMemorizer},
	} {
		err := q.client.XGroupCreateMkStream(ctx, pair.stream, pair.group, "0").Err()
		if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
			return fmt.Errorf("create group %s on %s: %w", pair.group, pair.stream, err)
		}
	}
	return nil
}

// PushTask adds a task message to the agent_tasks stream.
func (q *Queue) PushTask(ctx context.Context, msg TaskMessage) (string, error) {
	msgJSON, _ := json.Marshal(msg)
	result, err := q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: StreamTasks,
		Values: map[string]any{
			"turn_id":     msg.TurnID,
			"region_path": msg.RegionPath,
			"context_ref": msg.ContextRef,
			"task_type":   msg.TaskType,
			"prompt":      msg.Prompt,
			"review":      msg.Review,
			"payload":     string(msgJSON),
		},
	}).Result()
	if err != nil {
		return "", fmt.Errorf("push task: %w", err)
	}
	return result, nil
}

// PushProposal adds a proposal message to the agent_proposals stream.
func (q *Queue) PushProposal(ctx context.Context, msg ProposalMessage) (string, error) {
	result, err := q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: StreamProposals,
		Values: map[string]any{
			"turn_id":     msg.TurnID,
			"proposal_id": msg.ProposalID,
			"region_path": msg.RegionPath,
		},
	}).Result()
	if err != nil {
		return "", fmt.Errorf("push proposal: %w", err)
	}
	return result, nil
}

// ReadTask reads one task message from the agent_tasks stream (blocking).
func (q *Queue) ReadTask(ctx context.Context, consumer string) (*TaskMessage, string, error) {
	streams, err := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    GroupResearcher,
		Consumer: consumer,
		Streams:  []string{StreamTasks, ">"},
		Count:    1,
		Block:    0,
	}).Result()
	if err != nil {
		return nil, "", fmt.Errorf("read task: %w", err)
	}

	for _, stream := range streams {
		for _, msg := range stream.Messages {
			task := &TaskMessage{
				TurnID:     getString(msg.Values, "turn_id"),
				RegionPath: getString(msg.Values, "region_path"),
				ContextRef: getString(msg.Values, "context_ref"),
				TaskType:   getString(msg.Values, "task_type"),
				Prompt:     getString(msg.Values, "prompt"),
				Review:     getString(msg.Values, "review"),
			}
			return task, msg.ID, nil
		}
	}
	return nil, "", fmt.Errorf("no messages")
}

// ReadProposal reads one proposal message from the agent_proposals stream (blocking).
func (q *Queue) ReadProposal(ctx context.Context, consumer string) (*ProposalMessage, string, error) {
	streams, err := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    GroupMemorizer,
		Consumer: consumer,
		Streams:  []string{StreamProposals, ">"},
		Count:    1,
		Block:    0,
	}).Result()
	if err != nil {
		return nil, "", fmt.Errorf("read proposal: %w", err)
	}

	for _, stream := range streams {
		for _, msg := range stream.Messages {
			proposal := &ProposalMessage{
				TurnID:     getString(msg.Values, "turn_id"),
				ProposalID: getString(msg.Values, "proposal_id"),
				RegionPath: getString(msg.Values, "region_path"),
			}
			return proposal, msg.ID, nil
		}
	}
	return nil, "", fmt.Errorf("no messages")
}

// AckTask acknowledges a task message.
func (q *Queue) AckTask(ctx context.Context, msgID string) error {
	return q.client.XAck(ctx, StreamTasks, GroupResearcher, msgID).Err()
}

// AckProposal acknowledges a proposal message.
func (q *Queue) AckProposal(ctx context.Context, msgID string) error {
	return q.client.XAck(ctx, StreamProposals, GroupMemorizer, msgID).Err()
}

// Status returns pending message counts for both streams.
func (q *Queue) Status(ctx context.Context) (tasks, proposals int64, err error) {
	tasksLen, err := q.client.XLen(ctx, StreamTasks).Result()
	if err != nil {
		return 0, 0, err
	}
	proposalsLen, err := q.client.XLen(ctx, StreamProposals).Result()
	if err != nil {
		return 0, 0, err
	}
	return tasksLen, proposalsLen, nil
}

func getString(values map[string]any, key string) string {
	if v, ok := values[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
