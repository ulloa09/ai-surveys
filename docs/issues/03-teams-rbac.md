# 03 — Teams & RBAC (Team Management, Permission Enforcement)

## What to build

Introduce teams (departments/groups) and enforce fine-grained permissions at the API layer. Admins can create teams, invite members, and assign roles within a team. All survey-scoped resources are visible only to the owning team and super-admins by default. Viewers can read analysis but cannot edit or export.

Deliver:
- `teams` table with name, created_by, and members (user FK + team-scoped role: `admin`, `viewer`)
- Team creation and member invitation UI in the admin shell
- Permission enforcement middleware: survey-scoped endpoints check team membership before allowing access
- Super-admins bypass team scoping and can see all surveys across all teams
- Viewers can access the analysis dashboard but receive `403` on edit and export endpoints

## Acceptance criteria

- [ ] An admin can create a team and becomes its first member with `admin` role
- [ ] An admin can invite another user to a team by email; invitee's role defaults to `admin`, configurable to `viewer`
- [ ] A survey created by a team member is visible to all members of that team
- [ ] A user who is not a team member receives `403` on any endpoint scoped to that team's surveys
- [ ] A super-admin can list and access all surveys regardless of team
- [ ] A viewer team member can fetch analysis results but receives `403` on export and survey edit endpoints
- [ ] Team list and member management UI renders correctly in the admin shell

## Blocked by

- #02 Email & Password Authentication
