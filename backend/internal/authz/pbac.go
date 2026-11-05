package authz

import (
	"slices"
	"time"
	"uuid"
)

type Action string

// Action constants define the various actions that can be performed in the system.
const (
	// Challenges
	ChallengeRead        Action = "challenge:read"
	ChallengeDownload    Action = "challenge:download"
	ChallengeViewSolves  Action = "challenge:view_solves"
	ChallengeReadSecrets Action = "challenge:read_secrets"
	ChallengeCreate      Action = "challenge:create"
	ChallengeUpdate      Action = "challenge:update"
	ChallengeManageFiles Action = "challenge:manage_files"
	ChallengeDelete      Action = "challenge:delete"
	ChallengeSubmit      Action = "challenge:submit"

	// Instances
	InstanceStart  Action = "instance:start"
	InstanceStop   Action = "instance:stop"
	InstanceRead   Action = "instance:read"
	InstanceExtend Action = "instance:extend"
	InstanceReset  Action = "instance:reset"

	// Team
	TeamCreate          Action = "team:create"
	TeamJoin            Action = "team:join"
	TeamReadPrivate     Action = "team:read_private"
	TeamReadPublic      Action = "team:read_public"
	TeamUpdate          Action = "team:update"
	TeamLeave           Action = "team:leave"
	TeamKick            Action = "team:kick"
	TeamTransferCaptain Action = "team:transfer_captain"
	TeamDelete          Action = "team:delete"
	TeamBan             Action = "team:ban"

	// Users
	UserReadPrivate Action = "user:read_private"
	UserReadPublic  Action = "user:read_public"
	UserUpdate      Action = "user:update"
	UserManageKeys  Action = "user:manage_keys"
	UserCreate      Action = "user:create"
	UserBan         Action = "user:ban"
	UserDelete      Action = "user:delete"
	UserSetRole     Action = "user:set_role"
	UserRevokeKeys  Action = "user:revoke_keys"
	UserListKeys    Action = "user:list_keys"
	UserList        Action = "user:list"

	// Submissions
	SubmissionRead   Action = "submission:read"
	SubmissionDelete Action = "submission:delete"

	// Announcements
	AnnouncementCreate Action = "announcement:create"
	AnnouncementDelete Action = "announcement:delete"

	// Settings
	SettingsRead   Action = "settings:read"
	SettingsUpdate Action = "settings:update"

	// Misc
	AuditRead Action = "audit:read"
)

// Effect says whether a policy allows or denies the actions it matches.
type Effect string

const (
	Allow Effect = "allow"
	Deny  Effect = "deny"
)

// AnyAction matches every action.
const AnyAction Action = "*"

// Condition is a fact about the subject, resource or environment. A policy applies when all
// of its conditions hold.
type Condition string

// Condition constants define the various conditions that can be applied to actions.
const (
	Always        Condition = "always"
	IsSelf        Condition = "is_self"
	IsOwner       Condition = "is_owner"
	HasTeam       Condition = "has_team"
	NoTeam        Condition = "no_team"
	IsMember      Condition = "is_member"
	IsCaptain     Condition = "is_captain"
	SameTeam      Condition = "same_team"
	Published     Condition = "published"
	NeedsInstance Condition = "needs_instance"
	CTFStarted    Condition = "ctf_started"
	CTFRunning    Condition = "ctf_running"
	CTFEnded      Condition = "ctf_ended"

	// Used by deny policies.
	SubjectBanned     Condition = "subject_banned"
	TeamBanned        Condition = "team_banned"
	TargetIsLastAdmin Condition = "target_is_last_admin"
	TeamHasOthers     Condition = "team_has_others"
	TeamIsFull        Condition = "team_full"
	TargetBanned      Condition = "target_banned"
	NotAdmin          Condition = "not_admin"
)

// Subject is the user performing an action.
type Subject struct {
	ID         uuid.UUID // zero for a logged out visitor
	Role       Role
	Banned     bool
	TeamID     uuid.UUID // zero when the user has no team
	IsCaptain  bool      // captain of TeamID
	TeamBanned bool      // TeamID is banned
}

// Resource is the object an action is performed on. Fields that do not apply to the
// resource type are left empty, but every field a deny policy reads must be set, since a
// missing value makes that deny policy not apply.
type Resource struct {
	ID          uuid.UUID // the resource's own ID (a user ID for users, a team ID for teams)
	OwnerID     uuid.UUID // the user who created it (challenges)
	TeamID      uuid.UUID // the team it belongs to (instances)
	Visible     bool      // not hidden (challenges)
	DockerImage string    // set when the challenge needs an instance
	Role        Role      // the target user's role (users)
	MemberCount int       // number of members (teams)
	Banned      bool      // the resource itself is banned (teams)
}

// Env holds the state of the event that conditions depend on.
// A zero CTFStart or CTFEnd means it is not configured, and conditions depending on it fail.
type Env struct {
	Now         time.Time
	CTFStart    time.Time
	CTFEnd      time.Time
	AdminCount  int // number of users with RoleAdmin
	MaxTeamSize int // 0 means no limit
}

// ConditionCheckFunc reports whether condition holds for subject s acting on resource r.
// Unknown conditions fail.
func ConditionCheckFunc(condition Condition, s Subject, r Resource, env Env) bool {
	started := !env.CTFStart.IsZero() && !env.Now.Before(env.CTFStart)
	ended := !env.CTFEnd.IsZero() && !env.Now.Before(env.CTFEnd)
	hasTeam := s.TeamID != uuid.UUID{}
	isMember := hasTeam && s.TeamID == r.ID

	switch condition {
	case Always:
		return true
	case IsSelf:
		return s.ID != (uuid.UUID{}) && r.ID == s.ID
	case IsOwner:
		return s.ID != (uuid.UUID{}) && r.OwnerID == s.ID
	case HasTeam:
		return hasTeam
	case NoTeam:
		return !hasTeam
	case IsMember:
		return isMember
	case IsCaptain:
		return s.IsCaptain && isMember
	case SameTeam:
		return hasTeam && r.TeamID == s.TeamID
	case Published:
		return r.Visible
	case NeedsInstance:
		return r.DockerImage != ""
	case CTFStarted:
		return started
	case CTFRunning:
		return started && !env.CTFEnd.IsZero() && !ended
	case CTFEnded:
		return ended
	case SubjectBanned:
		return s.Banned
	case TeamBanned:
		return s.TeamBanned
	case TargetIsLastAdmin:
		return r.Role == RoleAdmin && env.AdminCount <= 1
	case TeamHasOthers:
		return r.MemberCount > 1
	case TargetBanned:
		return r.Banned
	case TeamIsFull:
		return env.MaxTeamSize > 0 && r.MemberCount >= env.MaxTeamSize
	case NotAdmin:
		return s.Role != RoleAdmin
	default:
		return false
	}
}

// Policy applies to Actions when every one of its Conditions holds, and then allows or denies
// them according to Effect.
type Policy struct {
	Name       string
	Effect     Effect
	Actions    []Action
	Conditions []Condition
}

// PublicRead lets anyone, including logged out visitors, read public profiles of users and teams.
var PublicRead = Policy{
	Name:       "PublicRead",
	Effect:     Allow,
	Actions:    []Action{UserReadPublic, TeamReadPublic},
	Conditions: []Condition{Always},
}

// Player policies.
var (
	PlayerChallengeRead = Policy{
		Name:       "PlayerChallengeRead",
		Effect:     Allow,
		Actions:    []Action{ChallengeRead, ChallengeDownload, ChallengeViewSolves},
		Conditions: []Condition{Published, CTFStarted},
	}
	PlayerSubmit = Policy{
		Name:       "PlayerSubmit",
		Effect:     Allow,
		Actions:    []Action{ChallengeSubmit},
		Conditions: []Condition{Published, HasTeam, CTFRunning},
	}
	TeamInstanceStart = Policy{
		Name:       "TeamInstanceStart",
		Effect:     Allow,
		Actions:    []Action{InstanceStart},
		Conditions: []Condition{Published, NeedsInstance, HasTeam, CTFRunning},
	}
	TeamInstanceControl = Policy{
		Name:       "TeamInstanceControl",
		Effect:     Allow,
		Actions:    []Action{InstanceRead, InstanceReset, InstanceExtend, InstanceStop},
		Conditions: []Condition{SameTeam, CTFRunning},
	}
	SelfAccount = Policy{
		Name:       "SelfAccount",
		Effect:     Allow,
		Actions:    []Action{UserReadPrivate, UserUpdate, UserManageKeys, UserListKeys, UserRevokeKeys},
		Conditions: []Condition{IsSelf},
	}
	TeamFounding = Policy{
		Name:       "TeamFounding",
		Effect:     Allow,
		Actions:    []Action{TeamCreate, TeamJoin},
		Conditions: []Condition{NoTeam},
	}
	TeamMember = Policy{
		Name:       "TeamMember",
		Effect:     Allow,
		Actions:    []Action{TeamReadPrivate, TeamLeave},
		Conditions: []Condition{IsMember},
	}
	TeamCaptain = Policy{
		Name:       "TeamCaptain",
		Effect:     Allow,
		Actions:    []Action{TeamUpdate, TeamKick, TeamTransferCaptain, TeamDelete},
		Conditions: []Condition{IsCaptain},
	}
)

// Admin and maintainer policies.
var (
	ChallengeAuthoring = Policy{
		Name:       "ChallengeAuthoring",
		Effect:     Allow,
		Actions:    []Action{ChallengeCreate},
		Conditions: []Condition{Always},
	}
	ChallengeOwner = Policy{
		Name:       "ChallengeOwner",
		Effect:     Allow,
		Actions:    []Action{ChallengeUpdate, ChallengeReadSecrets, ChallengeManageFiles},
		Conditions: []Condition{IsOwner},
	}
	ChallengeReadAll = Policy{
		Name:       "ChallengeReadAll",
		Effect:     Allow,
		Actions:    []Action{ChallengeRead, ChallengeDownload, ChallengeViewSolves},
		Conditions: []Condition{Always},
	}
	InstanceOperator = Policy{
		Name:       "InstanceOperator",
		Effect:     Allow,
		Actions:    []Action{InstanceRead, InstanceStop},
		Conditions: []Condition{Always},
	}
	ChallengeAdmin = Policy{
		Name:       "ChallengeAdmin",
		Effect:     Allow,
		Actions:    []Action{ChallengeUpdate, ChallengeReadSecrets, ChallengeManageFiles, ChallengeDelete},
		Conditions: []Condition{Always},
	}
	InstanceAdmin = Policy{
		Name:       "InstanceAdmin",
		Effect:     Allow,
		Actions:    []Action{InstanceRead, InstanceReset, InstanceStop, InstanceExtend},
		Conditions: []Condition{Always},
	}
	UserAdmin = Policy{
		Name:       "UserAdmin",
		Effect:     Allow,
		Actions:    []Action{UserList, UserReadPrivate, UserCreate, UserUpdate, UserBan, UserSetRole, UserRevokeKeys, UserDelete},
		Conditions: []Condition{Always},
	}
	TeamAdmin = Policy{
		Name:       "TeamAdmin",
		Effect:     Allow,
		Actions:    []Action{TeamReadPrivate, TeamUpdate, TeamBan, TeamKick, TeamDelete},
		Conditions: []Condition{Always},
	}
	SubmissionAdmin = Policy{
		Name:       "SubmissionAdmin",
		Effect:     Allow,
		Actions:    []Action{SubmissionRead, SubmissionDelete},
		Conditions: []Condition{Always},
	}
	EventAdmin = Policy{
		Name:       "EventAdmin",
		Effect:     Allow,
		Actions:    []Action{SettingsRead, SettingsUpdate, AnnouncementCreate, AnnouncementDelete, AuditRead},
		Conditions: []Condition{Always},
	}
)

// SystemPolicies apply to every subject regardless of role.
var SystemPolicies = []Policy{
	{Name: "DenyBanned", Effect: Deny, Actions: []Action{AnyAction}, Conditions: []Condition{SubjectBanned}},
	{Name: "DenyBannedTeam", Effect: Deny, Actions: []Action{ChallengeSubmit, InstanceStart, TeamKick}, Conditions: []Condition{TeamBanned}},
	{Name: "NoJoinBannedTeam", Effect: Deny, Actions: []Action{TeamJoin}, Conditions: []Condition{TargetBanned}},
	{Name: "NoSelfRoleChange", Effect: Deny, Actions: []Action{UserSetRole}, Conditions: []Condition{IsSelf}},
	{Name: "NoSelfDestruct", Effect: Deny, Actions: []Action{UserBan, UserDelete}, Conditions: []Condition{IsSelf}},
	{Name: "KeepLastAdmin", Effect: Deny, Actions: []Action{UserSetRole, UserBan, UserDelete}, Conditions: []Condition{TargetIsLastAdmin}},
	{Name: "CaptainMustTransfer", Effect: Deny, Actions: []Action{TeamLeave}, Conditions: []Condition{IsCaptain, TeamHasOthers}},
	{Name: "TeamFull", Effect: Deny, Actions: []Action{TeamJoin}, Conditions: []Condition{TeamIsFull}},
	{Name: "RosterFrozen", Effect: Deny, Actions: []Action{TeamJoin, TeamLeave, TeamKick}, Conditions: []Condition{CTFEnded, NotAdmin}},
}

// Role is the access level of a user.
type Role string

const (
	RoleAdmin      Role = "admin"
	RoleMaintainer Role = "maintainer"
	RoleUser       Role = "user"
)

// Policies attached to each role. Each role inherits every policy of the role below it:
// admin inherits maintainer, and maintainer inherits user.
var (
	userPolicies = []Policy{
		PlayerChallengeRead, PlayerSubmit, TeamInstanceStart, TeamInstanceControl,
		SelfAccount, TeamFounding, TeamMember, TeamCaptain,
	}
	maintainerPolicies = slices.Concat(userPolicies, []Policy{
		ChallengeAuthoring, ChallengeOwner, ChallengeReadAll, InstanceOperator,
	})
	adminPolicies = slices.Concat(maintainerPolicies, []Policy{
		ChallengeAdmin, InstanceAdmin, UserAdmin, TeamAdmin, SubmissionAdmin, EventAdmin,
	})
)

// RolePolicies maps each role to the policies attached to it, inherited ones included.
var RolePolicies = map[Role][]Policy{
	RoleUser:       userPolicies,
	RoleMaintainer: maintainerPolicies,
	RoleAdmin:      adminPolicies,
}

// EveryonePolicies apply to every subject, also one without a role (a logged out visitor).
var EveryonePolicies = []Policy{PublicRead}

// IsActionAllowed reports whether subject may perform action on resource in env. Any matching
// deny policy refuses. Otherwise the action is allowed when a matching allow policy in
// EveryonePolicies or attached to the subject's role applies. Nothing matching means refused.
func IsActionAllowed(subject Subject, action Action, resource Resource, env Env) bool {
	policies := slices.Concat(SystemPolicies, EveryonePolicies, RolePolicies[subject.Role])

	allowed := false
	for _, p := range policies {
		if !p.matches(action) || !allConditionsMet(p.Conditions, subject, resource, env) {
			continue
		}
		switch p.Effect {
		case Deny:
			return false
		case Allow:
			allowed = true
		}
	}
	return allowed
}

// allConditionsMet reports whether every condition holds. A policy without conditions never
// applies, so a policy must use Always to apply unconditionally.
func allConditionsMet(conditions []Condition, s Subject, r Resource, env Env) bool {
	if len(conditions) == 0 {
		return false
	}
	for _, c := range conditions {
		if !ConditionCheckFunc(c, s, r, env) {
			return false
		}
	}
	return true
}

// matches reports whether p lists a, or lists AnyAction.
func (p Policy) matches(a Action) bool {
	return slices.Contains(p.Actions, AnyAction) || slices.Contains(p.Actions, a)
}
