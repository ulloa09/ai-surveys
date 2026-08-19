<script lang="ts">
  import TeamCard from './TeamCard.svelte';
  import EmptyState from '../shared/EmptyState.svelte';
  import type { Team } from '$lib/types';

  let {
    teams,
    canManage = false,
    onCreateTeam = () => {}
  }: {
    teams: Team[];
    canManage?: boolean;
    onCreateTeam?: () => void;
  } = $props();
</script>

{#if teams.length === 0}
  <EmptyState
    title="No tienes equipos todavía"
    description="Crea un equipo para empezar a gestionar encuestas con tus colaboradores."
    actionLabel={canManage ? 'Crear equipo' : ''}
    onAction={onCreateTeam}
  />
{:else}
  <div class="grid">
    {#each teams as team}
      <TeamCard {team} {canManage} />
    {/each}
  </div>
{/if}

<style>
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
    gap: 1rem;
  }
</style>
