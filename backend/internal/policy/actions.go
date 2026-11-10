package policy

// Actions known to the backend. The loader rejects action patterns that match none of these,
// so a typo in a policy file fails at startup instead of silently never matching.
const (
	// Challenges
	ChallengeRead        = "challenge:read"
	ChallengeDownload    = "challenge:download"
	ChallengeViewSolves  = "challenge:view_solves"
	ChallengeReadSecrets = "challenge:read_secrets"
	ChallengeCreate      = "challenge:create"
	ChallengeUpdate      = "challenge:update"
	ChallengeManageFiles = "challenge:manage_files"
	ChallengeDelete      = "challenge:delete"
	ChallengeSubmit      = "challenge:submit"

	// Instances
	InstanceStart  = "instance:start"
	InstanceStop   = "instance:stop"
	InstanceRead   = "instance:read"
	InstanceExtend = "instance:extend"
	InstanceReset  = "instance:reset"

	// Teams
	TeamCreate          = "team:create"
	TeamJoin            = "team:join"
	TeamReadPrivate     = "team:read_private"
	TeamReadPublic      = "team:read_public"
	TeamUpdate          = "team:update"
	TeamLeave           = "team:leave"
	TeamKick            = "team:kick"
	TeamTransferCaptain = "team:transfer_captain"
	TeamDelete          = "team:delete"
	TeamBan             = "team:ban"

	// Users
	UserReadPrivate = "user:read_private"
	UserReadPublic  = "user:read_public"
	UserUpdate      = "user:update"
	UserManageKeys  = "user:manage_keys"
	UserCreate      = "user:create"
	UserBan         = "user:ban"
	UserDelete      = "user:delete"
	UserSetRole     = "user:set_role"
	UserRevokeKeys  = "user:revoke_keys"
	UserListKeys    = "user:list_keys"
	UserList        = "user:list"

	// Submissions
	SubmissionRead   = "submission:read"
	SubmissionDelete = "submission:delete"

	// Announcements
	AnnouncementCreate = "announcement:create"
	AnnouncementDelete = "announcement:delete"

	// Settings
	SettingsRead   = "settings:read"
	SettingsUpdate = "settings:update"

	// Misc
	AuditRead = "audit:read"
)

// Actions lists every known action.
var Actions = []string{
	ChallengeRead, ChallengeDownload, ChallengeViewSolves, ChallengeReadSecrets, ChallengeCreate,
	ChallengeUpdate, ChallengeManageFiles, ChallengeDelete, ChallengeSubmit,
	InstanceStart, InstanceStop, InstanceRead, InstanceExtend, InstanceReset,
	TeamCreate, TeamJoin, TeamReadPrivate, TeamReadPublic, TeamUpdate, TeamLeave, TeamKick,
	TeamTransferCaptain, TeamDelete, TeamBan,
	UserReadPrivate, UserReadPublic, UserUpdate, UserManageKeys, UserCreate, UserBan, UserDelete,
	UserSetRole, UserRevokeKeys, UserListKeys, UserList,
	SubmissionRead, SubmissionDelete,
	AnnouncementCreate, AnnouncementDelete,
	SettingsRead, SettingsUpdate,
	AuditRead,
}
