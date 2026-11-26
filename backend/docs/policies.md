# PBAC

Authorisation is policy based. The rules are JSON in [`internal/policies/default.json`](internal/policies/default.json), which `main` loads at startup from `POLICY_FILE`. A missing or invalid file stops the server. [`internal/policy`](internal/policy/engine.go) is the engine, which is pure Go with no database or HTTP. [`internal/auth`](internal/auth/authorise.go) collects the attributes a check needs and connects it to HTTP.

## How a check works

```
a.Authorise(action, h) / a.IsAllowed(r, u, action, resource)
  └─ a.Checker(r, u)
       ├─ a.Subject(ctx, u)   who: id, role, banned, team, captain, team banned    (users + teams)
       ├─ a.Env(ctx)          context: ctf_phase, max_team_size, admin_count, ...  (settings + users)
       └─ engine.Evaluate(request)
            1. a statement matches when its subjects, actions and resources patterns match
               and all of its conditions hold
            2. any matching deny statement  → refused
            3. any matching allow statement → allowed, otherwise refused
```

A resource's `Path` defaults to the request path (`r.URL.Path`), so statements' `resources` are API paths. A logged out visitor has no ID and no roles, so only statements for `"*"` apply. A refusal is `401` for visitors and `403` for logged-in users (`auth.Deny`). Refusals are logged at debug level with the deciding policy and statement.

The handler builds the `policy.Resource` (ID and attributes) from the row it loaded. **Every attribute a deny statement reads must be set**, because a missing attribute makes the condition fail and the deny not apply: `role` for users (`KeepLastAdmin`), and `member_count` and `banned` for teams (`CaptainMustTransfer`, `NoJoinBannedTeam`, `TeamJoinNotFull`). See `team.resource()` in [`internal/teams/store.go`](internal/teams/store.go).

## Policy file

```json
[
  {
    "id": "player",
    "statements": [
      {
        "sid": "TeamCaptain",
        "effect": "allow",
        "subjects": ["role:user", "role:maintainer", "role:admin"],
        "actions": ["team:update", "team:kick", "team:transfer_captain"],
        "resources": ["/api/teams/mine", "/api/teams/mine/*"],
        "conditions": [
          { "operator": "Equals", "key": "subject.is_captain", "value": true },
          { "operator": "Equals", "key": "resource.id", "value": "${subject.team_id}" }
        ]
      }
    ]
  }
]
```

- **subjects:** `role:<role>`, `user:<id>` or `*`. Roles don't inherit, so list every role a statement applies to.
- **actions:** see [Actions](#actions). `*` and `team:*` work too.
- **resources:** API paths. `*` matches any characters including `/`, so `/api/teams/*` doesn't match `/api/teams` itself; list both when needed.
- **conditions:** all must hold. Operators are `Equals`, `NotEquals`, `In` and `NotIn` (value is a list), `LessThan` and `GreaterThan`. A value `"${scope.attr}"` compares against another attribute. A condition on a missing attribute fails.

The loader rejects unknown fields, effects, operators, condition keys and actions at startup, so a typo can't silently disable a rule. [`internal/teams/routes_pbac_test.go`](internal/teams/routes_pbac_test.go) calls every route as every kind of user; add the new routes there when you change a rule.

## Attributes

| Key                     | Type   | Notes                                                       |
|-------------------------|--------|-------------------------------------------------------------|
| `subject.id`            | string | empty for a visitor                                         |
| `subject.role`          | string | `user`, `maintainer`, `admin`, or empty for a visitor       |
| `subject.banned`        | bool   |                                                             |
| `subject.has_team`      | bool   |                                                             |
| `subject.team_id`       | string | empty when not in a team                                    |
| `subject.is_captain`    | bool   | captain of `subject.team_id`                                |
| `subject.team_banned`   | bool   | `subject.team_id` is banned                                 |
| `resource.id`           | string | the loaded row's ID                                         |
| `resource.path`         | string | the request path                                            |
| `resource.*`            | any    | set by the handler, e.g. `member_count`, `banned`, `hidden` (teams), `visible`, `needs_instance`, `owner_id` (challenges), `team_id` (instances), `role` (users) |
| `context.ctf_phase`     | string | `not_started`, `running` or `ended`                         |
| `context.ctf_start`, `context.ctf_end`, `context.now` | number | Unix seconds                  |
| `context.max_team_size` | number |                                                             |
| `context.admin_count`   | number | users with role `admin`                                     |
| `request.action`        | string | the action being checked                                    |

## Actions

The constants live in [`internal/policy/actions.go`](internal/policy/actions.go). Add new actions there before using them in the policy file.

| Resource     | Actions                                                                                       |
|--------------|-----------------------------------------------------------------------------------------------|
| challenge    | read, download, view_solves, read_secrets, create, update, manage_files, delete, submit       |
| instance     | start, read, reset, extend, stop                                                              |
| team         | create, join, read_private, read_public, update, leave, kick, transfer_captain, delete, ban   |
| user         | read_private, read_public, update, manage_keys, create, ban, set_role, revoke_keys, delete, list, list_keys |
| submission   | read, delete                                                                                  |
| announcement | create, delete                                                                                |
| settings     | read, update                                                                                  |
| audit        | read                                                                                          |

## Default policies

| Policy       | Applies to                 | Statements                                                                 |
|--------------|----------------------------|----------------------------------------------------------------------------|
| `system-deny`| everyone                   | DenyBanned, DenyBannedTeam, NoJoinBannedTeam, NoSelfRoleChange, KeepLastAdmin, CaptainMustTransfer, RosterFrozen |
| `public`     | everyone, also visitors    | PublicRead                                                                 |
| `player`     | user, maintainer, admin    | PlayerChallengeRead, PlayerSubmit, TeamInstanceStart, TeamInstanceControl, SelfAccount, TeamCreate, TeamJoinUnlimited, TeamJoinNotFull, TeamMember, TeamCaptain |
| `maintainer` | maintainer, admin          | ChallengeAuthoring, ChallengeOwner, ChallengeReadAll, InstanceOperator     |
| `admin`      | admin                      | ChallengeAdmin, InstanceAdmin, UserAdmin, TeamAdmin, SubmissionAdmin, EventAdmin |

The deny statements:

| Statement           | Actions                                                       | Denies when                                           |
|---------------------|---------------------------------------------------------------|-------------------------------------------------------|
| DenyBanned          | * except user:read_public, team:read_public                   | the subject is banned                                 |
| DenyBannedTeam      | challenge:submit, instance:start, team:update, team:kick, team:transfer_captain, team:leave, team:delete | the subject's team is banned (player routes only) |
| NoJoinBannedTeam    | team:join                                                     | the team being joined is banned                       |
| NoSelfRoleChange    | user:set_role, user:ban, user:delete                          | the target user is the subject                        |
| KeepLastAdmin       | user:set_role, user:ban, user:delete                          | the target is an admin and there is only one          |
| CaptainMustTransfer | team:leave                                                    | the subject is captain and the team has other members |
| RosterFrozen        | team:join, team:leave, team:kick                              | the CTF has ended and the subject isn't an admin      |

A full team is handled on the allow side instead: `team:join` is allowed by `TeamJoinUnlimited` (`max_team_size` is 0) or `TeamJoinNotFull` (`member_count < max_team_size`), because the operators have no `>=`.

