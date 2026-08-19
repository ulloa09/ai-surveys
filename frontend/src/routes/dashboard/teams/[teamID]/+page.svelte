<script lang="ts">
  import { enhance } from '$app/forms';
  import { goto } from '$app/navigation';
  import type { PageData } from './$types';
  import { getPermissions } from '$lib/permissions';
  import TeamMembers from '$lib/components/teams/TeamMembers.svelte';

  let { data }: { data: PageData } = $props();

  const perms = $derived(getPermissions(data.user?.role ?? 'alumno'));
  const team  = $derived(data.team);
  const otherTeams = $derived(data.otherTeams ?? []);

  let showInviteForm = $state(false);
  let inviting       = $state(false);
  let inviteError    = $state('');

  let removing     = $state(false);
  let removeError  = $state('');
  let pendingRemoveUserID = $state('');

  let moving      = $state(false);
  let moveError   = $state('');
  let pendingMoveUserID   = $state('');
  let pendingMoveToTeamID = $state('');

  let memberSearch = $state(data.member ?? '');
  let memberSearchTimeout: ReturnType<typeof setTimeout>;

  function handleMemberSearch() {
    clearTimeout(memberSearchTimeout);
    memberSearchTimeout = setTimeout(() => {
      const url = new URL(window.location.href);
      if (memberSearch) { url.searchParams.set('member', memberSearch); }
      else              { url.searchParams.delete('member'); }
      goto(url.toString(), { replaceState: true, noScroll: true, keepFocus: true });
    }, 500);
  }

  // Dispara el submit del form oculto de remover, con el userID elegido.
  function requestRemove(userID: string) {
    pendingRemoveUserID = userID;
    removeError = '';
    setTimeout(() => {
      document.getElementById('form-remove')?.dispatchEvent(
        new Event('submit', { bubbles: true, cancelable: true })
      );
    }, 0);
  }

  // Dispara el submit del form oculto de mover, con el userID y equipo destino.
  function requestMove(userID: string, toTeamID: string) {
    pendingMoveUserID = userID;
    pendingMoveToTeamID = toTeamID;
    moveError = '';
    setTimeout(() => {
      document.getElementById('form-move')?.dispatchEvent(
        new Event('submit', { bubbles: true, cancelable: true })
      );
    }, 0);
  }
</script>

<svelte:head><title>{team?.name ?? 'Equipo'} — AI Surveys</title></svelte:head>

<!-- Form oculto para remover miembro -->
<form
  id="form-remove"
  method="POST"
  action="?/removeMember"
  style="display:none"
  use:enhance={() => {
    removing = true;
    return async ({ result, update }) => {
      removing = false;
      pendingRemoveUserID = '';
      if (result.type === 'failure') {
        removeError = (result.data as { removeError?: string })?.removeError ?? 'No se pudo remover al miembro';
      } else {
        removeError = '';
        await update();
      }
    };
  }}
>
  <input type="hidden" name="user_id" value={pendingRemoveUserID} />
</form>

<!-- Form oculto para mover miembro a otro equipo -->
<form
  id="form-move"
  method="POST"
  action="?/moveMember"
  style="display:none"
  use:enhance={() => {
    moving = true;
    return async ({ result, update }) => {
      moving = false;
      pendingMoveUserID = '';
      pendingMoveToTeamID = '';
      if (result.type === 'failure') {
        moveError = (result.data as { moveError?: string })?.moveError ?? 'No se pudo mover al miembro';
      } else {
        moveError = '';
        await update();
      }
    };
  }}
>
  <input type="hidden" name="user_id" value={pendingMoveUserID} />
  <input type="hidden" name="to_team_id" value={pendingMoveToTeamID} />
</form>

<div class="page">

  <div class="page-header">
    <div class="header-left">
      <a href="/dashboard/teams" class="back-link">← Equipos</a>
      <h1 class="page-title">{team?.name}</h1>
      <p class="page-sub">{team?.members?.length ?? 0} miembro(s)</p>
    </div>
    {#if perms.canInviteMember}
      <button class="btn-primary" onclick={() => showInviteForm = !showInviteForm}>
        {showInviteForm ? 'Cancelar' : '+ Invitar miembro'}
      </button>
    {/if}
  </div>

  {#if showInviteForm && perms.canInviteMember}
    <div class="form-card">
      <h2 class="form-title">Invitar miembro</h2>
      {#if inviteError}
        <p class="form-error">{inviteError}</p>
      {/if}
      <form
        method="POST"
        action="?/inviteMember"
        use:enhance={() => {
          inviting = true;
          inviteError = '';
          return async ({ result, update }) => {
            inviting = false;
            if (result.type === 'success') {
              showInviteForm = false;
              await update();
            } else if (result.type === 'failure') {
              inviteError = (result.data as { inviteError?: string })?.inviteError ?? 'Error al invitar';
            }
          };
        }}
        class="form-fields"
      >
        <div class="field">
          <label for="invite-email">Correo</label>
          <input
            id="invite-email"
            name="email"
            type="email"
            placeholder="nombre@example.com"
            required
            disabled={inviting}
          />
        </div>
        <button type="submit" class="btn-primary" disabled={inviting}>
          {inviting ? 'Invitando…' : 'Invitar'}
        </button>
      </form>
      <p class="form-hint">
        El rol (Profesor o Alumno) se asigna automáticamente según la cuenta del correo invitado.
      </p>
    </div>
  {/if}

  {#if removeError}
    <p class="form-error">{removeError}</p>
  {/if}
  {#if moveError}
    <p class="form-error">{moveError}</p>
  {/if}

  <div class="members-card">
    <div class="card-header">
      <h2 class="card-title">Miembros</h2>
      <input
        type="text"
        placeholder="Buscar por nombre o correo…"
        bind:value={memberSearch}
        oninput={handleMemberSearch}
        class="member-search"
      />
    </div>
    {#if memberSearch}
      <p class="results-count">{team?.members?.length ?? 0} resultado(s)</p>
    {/if}
    <TeamMembers
      members={team?.members ?? []}
      canRemove={perms.canRemoveMember}
      canMove={perms.canMoveMembers}
      otherTeams={otherTeams}
      onRemove={requestRemove}
      onMove={requestMove}
    />
  </div>

</div>


<style>
  .page {
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
    padding: 2rem;
    width: 100%;
  }

  .page-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 1rem;
  }

  .header-left {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  .back-link {
    font-size: 0.8rem;
    color: var(--muted);
    text-decoration: none;
    transition: color 0.15s;
  }

  .back-link:hover { color: var(--purple); }

  .page-title {
    font-size: 1.5rem;
    font-weight: 700;
    color: var(--text);
    margin: 0;
  }

  .page-sub {
    font-size: 0.875rem;
    color: var(--muted);
    margin: 0;
  }

  .btn-primary {
    background: var(--purple);
    color: white;
    border: none;
    border-radius: var(--radius-sm);
    padding: 0.5rem 1.25rem;
    font-size: 0.875rem;
    font-weight: 600;
    cursor: pointer;
    transition: opacity 0.15s;
    white-space: nowrap;
    flex-shrink: 0;
  }

  .btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }
  .btn-primary:not(:disabled):hover { opacity: 0.85; }

  .form-card {
    background: var(--white);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: 1.5rem;
    box-shadow: 0 2px 8px rgba(0,0,0,0.06);
  }

  .form-title {
    font-size: 1rem;
    font-weight: 600;
    color: var(--text);
    margin: 0 0 1rem;
  }

  .form-error {
    font-size: 0.8rem;
    color: #680f0f;
    background: #fef2f2;
    border: 1px solid #772e2e;
    border-radius: var(--radius-sm);
    padding: 0.5rem 0.75rem;
    margin-bottom: 1rem;
  }

  .form-fields {
    display: flex;
    gap: 0.75rem;
    align-items: flex-end;
    flex-wrap: wrap;
  }

  .field {
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
    flex: 1;
    min-width: 200px;
  }

  .form-hint {
    font-size: 0.75rem;
    color: var(--muted);
    margin: 0.75rem 0 0;
  }

  label {
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--muted);
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  input {
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    padding: 0.5rem 0.75rem;
    font-size: 0.875rem;
    color: var(--text);
    outline: none;
    transition: border-color 0.15s;
    width: 100%;
  }

  input:focus { border-color: var(--purple); }

  .members-card {
    background: var(--white);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: 1.5rem;
    box-shadow: 0 1px 4px rgba(0,0,0,0.04);
  }

  .member-search {
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    padding: 0.45rem 0.75rem;
    font-size: 0.875rem;
    color: var(--text);
    outline: none;
    transition: border-color 0.15s;
    max-width: 260px;
  }

  .member-search:focus { border-color: var(--purple); }

  .results-count {
    font-size: 0.8rem;
    color: var(--muted);
    margin: 0 0 1rem;
  }

  .card-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    margin-bottom: 1rem;
  }

  .card-title {
    font-size: 1rem;
    font-weight: 600;
    color: var(--text);
    margin: 0 0 1rem;
  }
</style>
