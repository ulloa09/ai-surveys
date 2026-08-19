<script lang="ts">
  import type { Survey } from '$lib/types';
  import SurveyCard from './SurveyCard.svelte';
  import EmptyState from '$lib/components/shared/EmptyState.svelte';

  let {
    surveys,
    canEdit = false,
    canDelete = false,
    canDuplicate = false,
    onDelete = (_id: string) => {},
    onDuplicate = (_id: string) => {}
  }: {
    surveys: Survey[];
    canEdit?: boolean;
    canDelete?: boolean;
    canDuplicate?: boolean;
    onDelete?: (id: string) => void;
    onDuplicate?: (id: string) => void;
  } = $props();
</script>

{#if surveys.length === 0}
  <EmptyState
    title="No hay encuestas todavía"
    description="Crea tu primera encuesta para empezar a recopilar retroalimentación."
  />
{:else}
  <div class="grid">
    {#each surveys as survey}
      <SurveyCard
        {survey}
        {canEdit}
        {canDelete}
        {canDuplicate}
        {onDelete}
        {onDuplicate}
      />
    {/each}
  </div>
{/if}

<style>
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
    gap: 1rem;
  }
</style>
