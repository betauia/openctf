# PBAC

## Conditions

| Condition      | True when                            |
|----------------|--------------------------------------|
| always         | always                               |
| is_self        | resource.id == subject.id            |
| is_owner       | resource.created_by_id == subject.id |
| has_team       | subject.team_id is set               |
| no_team        | subject.team_id is not set           |
| is_member      | subject.team_id == resource.id       |
| is_captain     | subject.is_captain and is_member     |
| same_team      | resource.team_id == subject.team_id  |
| published      | not resource.hidden                  |
| needs_instance | the challenge has a docker_image     |
| ctf_started    | now >= ctf_start                     |
| ctf_running    | ctf_start <= now < ctf_end           |
| ctf_ended      | now >= ctf_end                       |

## Actions

| Resource     | Actions                                                                                 |
|--------------|-----------------------------------------------------------------------------------------|
| challenge    | read, download, view_solves, read_secrets, create, update, manage_files, delete, submit |
| instance     | start, read, reset, extend, stop                                                        |
| team         | create, join, read_private, update, leave, kick, transfer_captain, disband, ban         |
| user         | read, update, manage_keys, create, ban, set_role, revoke_keys, delete, list             |
| submission   | read, delete                                                                            |
| announcement | create, delete                                                                          |
| settings     | read, update                                                                            |
| audit_log    | read                                                                                    |

## Allow policies

### Player Policies

| Policy              | Action                                                        | Resource  | Conditions                                       |
|---------------------|---------------------------------------------------------------|-----------|--------------------------------------------------|
| PlayerChallengeRead | challenge:read, challenge:download, challenge:view_solves     | Challenge | published, ctf_started                           |
| PlayerSubmit        | challenge:submit                                              | Challenge | published, has_team, ctf_running                 |
| TeamInstanceStart   | instance:start                                                | Challenge | published, needs_instance, has_team, ctf_running |
| TeamInstanceControl | instance:read, instance:reset, instance:extend, instance:stop | Instance  | same_team, ctf_running                           |
| SelfAccount         | user:read, user:update, user:manage_keys                      | User      | is_self                                          |
| TeamFounding        | team:create, team:join                                        | Team      | no_team                                          |
| TeamMember          | team:read_private, team:leave                                 | Team      | is_member                                        |
| TeamCaptain         | team:update, team:kick, team:transfer_captain, team:disband   | Team      | is_captain                                       |

### Admin policies

| Policy             | Action                                                                                                 | Resource   | Conditions |
|--------------------|--------------------------------------------------------------------------------------------------------|------------|------------|
| ChallengeAuthoring | challenge:create                                                                                       | Challenge  | always     |
| ChallengeOwner     | challenge:update, challenge:read_secrets, challenge:manage_files                                       | Challenge  | is_owner   |
| ChallengeReadAll   | challenge:read, challenge:download, challenge:view_solves                                              | Challenge  | always     |
| InstanceOperator   | instance:read, instance:stop                                                                           | Instance   | always     |
| ChallengeAdmin     | challenge:update, challenge:read_secrets, challenge:manage_files, challenge:delete                     | Challenge  | always     |
| InstanceAdmin      | instance:read, instance:reset, instance:stop                                                           | Instance   | always     |
| UserAdmin          | user:list, user:read, user:create, user:update, user:ban, user:set_role, user:revoke_keys, user:delete | User       | always     |
| TeamAdmin          | team:read_private, team:update, team:ban, team:kick, team:disband                                      | Team       | always     |
| SubmissionAdmin    | submission:read, submission:delete                                                                     | Submission | always     |
| EventAdmin         | settings:read, settings:update, announcement:create, announcement:delete, audit_log:read               | -          | always     |

### Role Mapping

| Role       | Attached policies                                                                                                                    |
|------------|--------------------------------------------------------------------------------------------------------------------------------------|
| user       | PlayerChallengeRead, PlayerSubmit, TeamInstanceStart, TeamInstanceControl, SelfAccount, TeamFounding, TeamMember, TeamCaptain       |
| maintainer | SelfAccount, ChallengeAuthoring, ChallengeOwner, ChallengeReadAll, InstanceOperator                                                 |
| admin      | SelfAccount, ChallengeAuthoring, ChallengeReadAll, ChallengeAdmin, InstanceAdmin, UserAdmin, TeamAdmin, SubmissionAdmin, EventAdmin |

### System Policies (always active)

| Policy              | Effect | Action                               | Denies when                                           |
|---------------------|--------|--------------------------------------|-------------------------------------------------------|
| DenyBanned          | Deny   | *                                    | subject.banned                                        |
| DenyBannedTeam      | Deny   | challenge:submit, instance:start     | the subject's team is banned                          |
| NoSelfRoleChange    | Deny   | user:set_role                        | the target user is the subject                        |
| NoSelfDestruct      | Deny   | user:ban, user:delete                | the target user is the subject                        |
| KeepLastAdmin       | Deny   | user:set_role, user:ban, user:delete | the target is the only admin                          |
| CaptainMustTransfer | Deny   | team:leave                           | the subject is captain and the team has other members |
| TeamFull            | Deny   | team:join                            | member_count >= settings.max_team_size                |
| RosterFrozen        | Deny   | team:join, team:leave, team:kick     | ctf_ended, and the subject isn't an admin             |

# API Endpoints

## Authentication

## Users

| Path            | Who         | Purpose        |
|-----------------|-------------|----------------|
| GET /users/{id} | Public      | Public profile |
| GET /me         | user:read   | Own profile    |
| PATCH /me       | user:update | Update profile |

## Administrator Endpoints

| Path                                       | Who                    | Purpose                       |
|--------------------------------------------|------------------------|-------------------------------|
| GET /admin/users                           | user:list              | List and search users         |
| POST /admin/users                          | user:create            | Create a user                 |
| GET /admin/users/{id}                      | user:read              | Full details about user       |
| PATCH /admin/users/{id}                    | user:update + user:ban | Edit or ban user              |
| PUT /admin/users/{id}/role                 | user:set_role          | Change role                   |
| DELETE /admin/users/{id}                   | user:delete            | Delete user                   |
| GET /admin/teams                           | team:read_private      | List and search teams         |
| GET /admin/teams/{id}                      | team:read_private      | Full details about team       |
| PATCH /admin/teams/{id}                    | team:update + team:ban | Edit any team. Ban a team     |
| DELETE /admin/teams/{id}/members/{uid}     | team:kick              | Remove member from team       |
| DELETE /admin/teams/{id}                   | team:disband           | Delete team                   |
| GET /admin/instances                       | instance:read          | List of all running instances |
| DELETE /admin/instances/{id}               | instance:stop          | Kill any instance             |
| GET /admin/submissions                     | submission:read        | View all flag attempts        |
| DELETE /admin/submissions/{id}             | submission:delete      | Delete a flag attempt         |
| POST /admin/announcements                  | announcement:create    | Create announcements          |
| DELETE /admin/announcements/{id}           | announcement:delete    | Delete announcement           |
| GET /admin/audit-log                       | audit_log:read         | View audit logs               |
| GET /admin/settings                        | settings:read          | Get current settings          |
| PATCH /admin/settings                      | settings:update        | CTF start/end, themes         |
| POST /admin/challenges/{cid}/files         | challenge:manage_files | Upload files to a challenge   |
| POST /admin/challenges                     | challenge:create       | Create a challenge            |
| PATCH /admin/challenges/{cid}              | challenge:update       | Edit a challenge              |
| DELETE /admin/challenges/{cid}             | challenge:delete       | Delete a challenge            |
| DELETE /admin/challenges/{cid}/files/{fid} | challenge:manage_files | Delete files from a challenge |

## Teams

| Path                             | Who                   | Purpose                                         |
|----------------------------------|-----------------------|-------------------------------------------------|
| GET /teams                       | Public                | List and search all visible teams               |
| GET /teams/{id}                  | Public                | Public team page                                |
| POST /teams                      | team:create           | Create team                                     |
| GET /teams/mine                  | team:read_private     | Your team / invite code                         |
| PATCH /teams/mine                | team:update           | Rename team, edit team fields                   |
| POST /teams/join                 | team:join             | Join a team                                     |
| POST /teams/mine/leave           | team:leave            | Leave the team. Captain must transfer first     |
| DELETE /teams/mine/members/{uid} | team:kick             | Kick team member                                |
| PUT /teams/mine/captain          | team:transfer_captain | Change team captain                             |

## Challenges

| Path                              | Who                   | Purpose                        |
|-----------------------------------|-----------------------|--------------------------------|
| GET /challenges                   | challenge:read        | List visible challenges        |
| GET /challenges/categories        | challenge:read        | List visible categories        |
| GET /challenges/{cid}             | challenge:read        | Challenge details              |
| GET /challenges/{cid}/solves      | challenge:view_solves | Which teams have solved it     |
| GET /challenges/{cid}/files/{fid} | challenge:download    | Download attachments           |
| POST /challenges/{cid}/submit     | challenge:submit      | Submit flag for this challenge |
| POST /challenges/submit           | challenge:submit      | Submit flag for any challenge  |

## Instances

| Path                                   | Who             | Purpose                         |
|----------------------------------------|-----------------|---------------------------------|
| GET /challenges/{cid}/instance         | instance:read   | Info about your team's instance |
| POST /challenges/{cid}/instance        | instance:start  | Start your team's instance      |
| POST /challenges/{cid}/instance/reset  | instance:reset  | Restart your team's instance    |
| POST /challenges/{cid}/instance/extend | instance:extend | Push the expiry time back       |
| DELETE /challenges/{cid}/instance      | instance:stop   | Stop team instance              |

## Scoreboard

| Path                    | Who    | Purpose                 |
|-------------------------|--------|-------------------------|
| GET /scoreboard         | Public | Team ranking and score  |
| GET /scoreboard/history | Public | Score over time (graph) |