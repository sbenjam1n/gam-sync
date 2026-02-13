package gam

import "time"

// Concept represents a self-contained unit of user-facing functionality.
type Concept struct {
	ID           string       `json:"id" db:"id"`
	Name         string       `json:"name" db:"name"`
	Purpose      string       `json:"purpose" db:"purpose"`
	Spec         ConceptSpec  `json:"spec"`
	StateMachine StateMachine `json:"state_machine"`
	Invariants   []Invariant  `json:"invariants"`
	CreatedAt    time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at" db:"updated_at"`
}

// ConceptSpec contains the full concept specification.
type ConceptSpec struct {
	TypeParams           []string                  `json:"type_params"`
	State                map[string]StateComponent `json:"state"`
	Actions              map[string]ActionSpec     `json:"actions"`
	OperationalPrinciple string                    `json:"operational_principle"`
}

// StateComponent describes a relational state component.
type StateComponent struct {
	Type string `json:"type"` // "set", "map"
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
	Of   string `json:"of,omitempty"`
}

// ActionSpec describes a named operation with typed input/output.
type ActionSpec struct {
	Cases []ActionCase `json:"cases"`
}

// ActionCase describes one case of an action (success vs error outputs).
type ActionCase struct {
	Input       map[string]string `json:"input"`
	Output      map[string]string `json:"output"`
	Description string            `json:"description"`
}

// StateMachine defines states and transitions for a concept.
type StateMachine struct {
	States      []string     `json:"states"`
	Transitions []Transition `json:"transitions"`
}

// Transition describes a legal state transition via an action.
type Transition struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Action string `json:"action"`
}

// Invariant is a rule that must always hold for a concept.
type Invariant struct {
	Name   string         `json:"name"`
	Rule   string         `json:"rule,omitempty"`
	Config map[string]any `json:"config,omitempty"`
	Type   string         `json:"type"` // representation, abstract, api, migration, dependency
}

// Synchronization is a declarative rule expressing inter-concept behavior.
type Synchronization struct {
	ID          string         `json:"id" db:"id"`
	Name        string         `json:"name" db:"name"`
	WhenClause  []WhenPattern  `json:"when_clause"`
	WhereClause []WherePattern `json:"where_clause,omitempty"`
	ThenClause  []ThenAction   `json:"then_clause"`
	Description string         `json:"description,omitempty"`
	Enabled     bool           `json:"enabled" db:"enabled"`
	CreatedAt   time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at" db:"updated_at"`
}

// WhenPattern matches action completions in a sync's when clause.
type WhenPattern struct {
	Concept     string            `json:"concept"`
	Action      string            `json:"action"`
	InputMatch  map[string]string `json:"input_match"`
	OutputMatch map[string]string `json:"output_match"`
}

// WherePattern queries concept states and binds variables.
type WherePattern struct {
	Concept  string            `json:"concept"`
	Pattern  map[string]any    `json:"pattern"`
	Optional bool              `json:"optional,omitempty"`
	Bind     map[string]string `json:"bind,omitempty"`
	Filter   string            `json:"filter,omitempty"`
}

// ThenAction invokes an action on a concept with bound variables.
type ThenAction struct {
	Concept string            `json:"concept"`
	Action  string            `json:"action"`
	Args    map[string]string `json:"args"`
}

// Proposal is a structured change request emitted by a Researcher agent.
type Proposal struct {
	ID               string           `json:"id" db:"id"`
	TurnID           string           `json:"turn_id" db:"turn_id"`
	RegionID         string           `json:"region_id" db:"region_id"`
	RegionPath       string           `json:"region_path"`
	ActionTaken      string           `json:"action_taken" db:"action_taken"`
	CurrentState     string           `json:"current_state"`
	ProposedState    string           `json:"proposed_state" db:"proposed_state"`
	SyncChanges      *SyncChanges     `json:"sync_changes,omitempty"`
	Evidence         ProposalEvidence `json:"evidence"`
	DeferredActions  []DeferredAction `json:"deferred_actions"`
	Status           string           `json:"status" db:"status"`
	ReviewIterations int              `json:"review_iterations" db:"review_iterations"`
	ReviewHistory    []ReviewComment  `json:"review_history"`
	ErrorCode        *int             `json:"validation_error_code"`
	ViolationDetails any              `json:"violation_details"`
	RejectionReason  string           `json:"rejection_reason,omitempty"`
	BranchName       string           `json:"branch_name" db:"branch_name"`
	CommitSHA        string           `json:"commit_sha" db:"commit_sha"`
	CreatedAt        time.Time        `json:"created_at" db:"created_at"`
}

// SyncChanges tracks synchronization modifications in a proposal.
type SyncChanges struct {
	Added    []Synchronization `json:"added,omitempty"`
	Modified []Synchronization `json:"modified,omitempty"`
	Deleted  []string          `json:"deleted,omitempty"`
}

// ProposalEvidence contains structured analysis blocks for validation.
type ProposalEvidence struct {
	APIAnalysis        *APIAnalysis        `json:"api_analysis,omitempty"`
	MigrationAnalysis  *MigrationAnalysis  `json:"migration_analysis,omitempty"`
	DependencyAnalysis *DependencyAnalysis `json:"dependency_analysis,omitempty"`
	ModifiedRegions    []ModifiedRegion    `json:"modified_regions"`
	Summary            string              `json:"summary"`
}

// APIAnalysis tracks changes to a concept's exported API surface.
type APIAnalysis struct {
	ExportsBefore []string `json:"exports_before"`
	ExportsAfter  []string `json:"exports_after"`
	Removals      []string `json:"removals"`
	Additions     []string `json:"additions"`
}

// MigrationAnalysis tracks database migration operations.
type MigrationAnalysis struct {
	Operations []string `json:"operations"`
	Reversible bool     `json:"reversible"`
	DataLoss   bool     `json:"data_loss"`
}

// DependencyAnalysis tracks dependency changes.
type DependencyAnalysis struct {
	Added   []string `json:"added"`
	Removed []string `json:"removed"`
	Changed []string `json:"changed"`
}

// ModifiedRegion tracks a region modified by a proposal.
type ModifiedRegion struct {
	Path        string `json:"path"`
	File        string `json:"file"`
	Description string `json:"description"`
	Hash        string `json:"hash"`
}

// DeferredAction is a task queued for a separate Researcher session.
type DeferredAction struct {
	TaskType     string `json:"task_type"`
	Reason       string `json:"reason"`
	TargetRegion string `json:"target_region"`
}

// Turn represents one bounded agent session.
type Turn struct {
	ID          string     `json:"id" db:"id"`
	AgentID     string     `json:"agent_id" db:"agent_id"`
	AgentRole   string     `json:"agent_role" db:"agent_role"`
	ScopePath   string     `json:"scope_path" db:"scope_path"`
	PlanID      string     `json:"plan_id,omitempty" db:"plan_id"`
	TaskType    string     `json:"task_type" db:"task_type"`
	Scratchpad  string     `json:"scratchpad" db:"scratchpad"`
	Status      string     `json:"status" db:"status"`
	TreeBefore  any        `json:"tree_before" db:"tree_before"`
	TreeAfter   any        `json:"tree_after" db:"tree_after"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	CompletedAt *time.Time `json:"completed_at" db:"completed_at"`
}

// FlowEntry records an action in the runtime provenance log.
type FlowEntry struct {
	ID          string    `json:"id" db:"id"`
	FlowToken   string    `json:"flow_token" db:"flow_token"`
	ConceptName string    `json:"concept_name" db:"concept_name"`
	ActionName  string    `json:"action_name" db:"action_name"`
	InputArgs   any       `json:"input_args" db:"input_args"`
	OutputArgs  any       `json:"output_args" db:"output_args"`
	SyncName    string    `json:"sync_name,omitempty" db:"sync_name"`
	ParentID    string    `json:"parent_id,omitempty" db:"parent_id"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// LifecycleHook is a registered hook that fires during turn lifecycle events.
type LifecycleHook struct {
	ID       string `json:"id" db:"id"`
	Event    string `json:"event" db:"event"`
	HookName string `json:"hook_name" db:"hook_name"`
	Priority int    `json:"priority" db:"priority"`
	Handler  string `json:"handler" db:"handler"`
	Config   any    `json:"config" db:"config"`
	Enabled  bool   `json:"enabled" db:"enabled"`
	Scope    string `json:"scope,omitempty" db:"scope"`
}

// ValidationResult is the outcome of running a validation tier.
type ValidationResult struct {
	Tier    int                `json:"tier"`
	Passed  bool              `json:"passed"`
	Code    int               `json:"code"`
	Message string            `json:"message"`
	Details []ValidationDetail `json:"details,omitempty"`
}

// ValidationDetail describes a single validation check result.
type ValidationDetail struct {
	Check    string `json:"check"`
	Passed   bool   `json:"passed"`
	Expected string `json:"expected,omitempty"`
	Got      string `json:"got,omitempty"`
	Fix      string `json:"fix,omitempty"` // MANDATORY for non-passing checks
}

// ExecutionPlan spans multiple turns toward a single goal.
type ExecutionPlan struct {
	ID           string     `json:"id" db:"id"`
	Name         string     `json:"name" db:"name"`
	Goal         string     `json:"goal" db:"goal"`
	Status       string     `json:"status" db:"status"`
	Decisions    []Decision `json:"decisions"`
	QualityGrade string     `json:"quality_grade" db:"quality_grade"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	CompletedAt  *time.Time `json:"completed_at" db:"completed_at"`
}

// Decision records a design choice made during an execution plan.
type Decision struct {
	Description  string   `json:"description"`
	Rationale    string   `json:"rationale"`
	Alternatives []string `json:"alternatives"`
	TurnID       string   `json:"turn_id"`
	DecidedAt    string   `json:"decided_at"`
}

// PlanTurn is a turn within an execution plan.
type PlanTurn struct {
	PlanID     string   `json:"plan_id" db:"plan_id"`
	TurnID     string   `json:"turn_id" db:"turn_id"`
	RegionPath string   `json:"region_path" db:"region_path"`
	Ordering   int      `json:"ordering" db:"ordering"`
	DependsOn  []string `json:"depends_on" db:"depends_on"`
	Status     string   `json:"status" db:"status"`
}

// QualityGrade is a per-region quality assessment.
type QualityGrade struct {
	RegionID   string    `json:"region_id" db:"region_id"`
	Category   string    `json:"category" db:"category"`
	Grade      string    `json:"grade" db:"grade"`
	Details    any       `json:"details" db:"details"`
	AssessedAt time.Time `json:"assessed_at" db:"assessed_at"`
	AssessedBy string    `json:"assessed_by" db:"assessed_by"`
}

// GoldenPrinciple is a mechanical rule for codebase coherence.
type GoldenPrinciple struct {
	ID          string `json:"id" db:"id"`
	Name        string `json:"name" db:"name"`
	Rule        string `json:"rule" db:"rule"`
	LintCheck   string `json:"lint_check" db:"lint_check"`
	Remediation string `json:"remediation" db:"remediation"`
	Enabled     bool   `json:"enabled" db:"enabled"`
}

// ReviewComment is feedback from the Tier 3 review loop.
type ReviewComment struct {
	ProposalID  string `json:"proposal_id"`
	Tier        int    `json:"tier"`
	Iteration   int    `json:"iteration"`
	Concern     string `json:"concern"`
	Remediation string `json:"remediation"`
	Severity    string `json:"severity"` // request_changes, reject, escalate_human
}

// Region represents a namespace scope marker.
type Region struct {
	ID             string    `json:"id" db:"id"`
	Path           string    `json:"path" db:"path"`
	Description    string    `json:"description" db:"description"`
	LifecycleState string   `json:"lifecycle_state" db:"lifecycle_state"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// TurnRegion logs which regions were touched during a turn.
type TurnRegion struct {
	TurnID   string `json:"turn_id" db:"turn_id"`
	RegionID string `json:"region_id" db:"region_id"`
	Action   string `json:"action" db:"action"` // created, modified, deleted
}

// SyncRef is an index entry for impact analysis of synchronizations.
type SyncRef struct {
	SyncID      string `json:"sync_id" db:"sync_id"`
	ConceptName string `json:"concept_name" db:"concept_name"`
	ActionName  string `json:"action_name" db:"action_name"`
	StateField  string `json:"state_field" db:"state_field"`
	ClauseType  string `json:"clause_type" db:"clause_type"` // when, where, then
}
