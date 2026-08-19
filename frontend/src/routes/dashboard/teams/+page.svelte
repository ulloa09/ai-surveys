<script lang="ts">
  import { enhance } from '$app/forms';
  import { goto } from '$app/navigation';
  import type { PageData } from './$types';
  import { getPermissions } from '$lib/permissions';
  import TeamList from '$lib/components/teams/TeamList.svelte';

  let { data }: { data: PageData } = $props();

  const perms = $derived(getPermissions(data.user?.role ?? 'alumno'));

  let showCreateForm = $state(false);
  let creating       = $state(false);
  let createError    = $state('');

  // Filtros — se inicializan con lo que ya viene en la URL para que
  // al recargar la pagina los controles muestren el estado actual.
  let search = $state(data.name ?? '');
  $effect(() => { search = data.name ?? ''; });
  let order  = $state(data.order ?? 'desc');
  let year   = $state(data.year  ?? '');
  let month  = $state(data.month ?? '');

  // Años disponibles desde 2024 hasta el año actual
  const currentYear = new Date().getFullYear();
  const years = Array.from({ length: currentYear - 2023 }, (_, i) => 2024 + i);

  const months = [
    { value: '1',  label: 'Enero' },
    { value: '2',  label: 'Febrero' },
    { value: '3',  label: 'Marzo' },
    { value: '4',  label: 'Abril' },
    { value: '5',  label: 'Mayo' },
    { value: '6',  label: 'Junio' },
    { value: '7',  label: 'Julio' },
    { value: '8',  label: 'Agosto' },
    { value: '9',  label: 'Septiembre' },
    { value: '10', label: 'Octubre' },
    { value: '11', label: 'Noviembre' },
    { value: '12', label: 'Diciembre' },
  ];

  let searchTimeout: ReturnType<typeof setTimeout>;

  // Construye la URL con todos los filtros activos y navega.
  // Se usa en todos los controles para mantener los filtros sincronizados.
  function applyFilters() {
    const url = new URL(window.location.href);
    if (search) { url.searchParams.set('name', search); }
    else         { url.searchParams.delete('name'); }
    if (order && order !== 'desc') { url.searchParams.set('order', order); }
    else                           { url.searchParams.delete('order'); }
    if (year)  { url.searchParams.set('year', year); }
    else       { url.searchParams.delete('year'); }
    if (month) { url.searchParams.set('month', month); }
    else       { url.searchParams.delete('month'); }
    goto(url.toString(), { replaceState: true, noScroll: true, keepFocus: true });
  }

  // El buscador espera 500ms para no disparar una llamada por cada letra.
  function handleSearch() {
    clearTimeout(searchTimeout);
    searchTimeout = setTimeout(applyFilters, 500);
  }

  // Los selectores aplican inmediatamente al cambiar.
  function handleFilterChange() {
    applyFilters();
  }

  function clearFilters() {
    search = '';
    order  = 'desc';
    year   = '';
    month  = '';
    goto('/dashboard/teams', { replaceState: true, noScroll: true });
  }

  const hasActiveFilters = $derived(search || year || month || order !== 'desc');
</script>

<svelte:head><title>Equipos — AI Surveys</title></svelte:head>

<div class="page">

  <!-- Banda institucional de título -->
  <div class="page-header">
    <div class="page-header-inner">
    <div>
      <h1 class="page-title">Equipos</h1>
      <p class="page-sub">
        {#if data.user?.role === 'super_admin' || data.user?.role === 'admin'}
          Todos los equipos de la plataforma
        {:else}
          Los equipos en los que participas
        {/if}
      </p>
    </div>
    {#if perms.canCreateTeam}
      <button class="btn-band" onclick={() => showCreateForm = !showCreateForm}>
        {showCreateForm ? 'Cancelar' : '+ Nuevo equipo'}
      </button>
    {/if}
    </div>
  </div>

  <div class="page-content">

  <!-- Filtros -->
  <div class="filters">
    <input
      type="text"
      placeholder="Buscar por nombre…"
      bind:value={search}
      oninput={handleSearch}
      class="filter-input filter-search"
    />

    <select bind:value={order} onchange={handleFilterChange} class="filter-select">
      <option value="desc">Más recientes primero</option>
      <option value="asc">Más antiguos primero</option>
    </select>

    <select bind:value={year} onchange={handleFilterChange} class="filter-select">
      <option value="">Todos los años</option>
      {#each years as y}
        <option value={String(y)}>{y}</option>
      {/each}
    </select>

    <select bind:value={month} onchange={handleFilterChange} class="filter-select">
      <option value="">Todos los meses</option>
      {#each months as m}
        <option value={m.value}>{m.label}</option>
      {/each}
    </select>

    {#if hasActiveFilters}
      <button class="btn-clear" onclick={clearFilters}>✕ Limpiar filtros</button>
    {/if}
  </div>

  <!-- Contador de resultados -->
  {#if hasActiveFilters}
    <p class="results-count">
      {data.teams?.length ?? 0} equipo(s) encontrado(s)
    </p>
  {/if}

  <!-- Formulario crear equipo -->
  {#if showCreateForm && perms.canCreateTeam}
    <div class="form-card">
      <h2 class="form-title">Crear equipo</h2>
      {#if createError}
        <p class="form-error">{createError}</p>
      {/if}
      <form
        method="POST"
        action="?/createTeam"
        use:enhance={() => {
          creating = true;
          createError = '';
          return async ({ result, update }) => {
            creating = false;
            if (result.type === 'success') {
              showCreateForm = false;
              await update();
            } else if (result.type === 'failure') {
              createError = (result.data as { error?: string })?.error ?? 'Error al crear equipo';
            }
          };
        }}
        class="form-fields"
      >
        <div class="field">
          <label for="team-name">Nombre del equipo</label>
          <input
            id="team-name"
            name="name"
            type="text"
            placeholder="Ej. Departamento de Ingeniería"
            required
            disabled={creating}
          />
        </div>
        <button type="submit" class="btn-primary" disabled={creating}>
          {creating ? 'Creando…' : 'Crear equipo'}
        </button>
      </form>
    </div>
  {/if}

  <!-- Lista de equipos -->
  <div class="teams-container">
    <TeamList
      teams={data.teams ?? []}
      canManage={perms.canInviteMember}
      onCreateTeam={() => showCreateForm = true}
    />
  </div>

  </div>
</div>

<style>
  .page {
    display: flex;
    flex-direction: column;
    width: 100%;
  }

  .page-header {
    background: var(--blue-700);
    padding: 2.25rem 2rem;
  }

  .page-header-inner {
    max-width: var(--container);
    margin: 0 auto;
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 1rem;
  }

  .page-content {
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
    padding: 2rem;
    width: 100%;
    max-width: var(--container);
    margin: 0 auto;
  }

  .page-title {
    font-size: 1.5rem;
    font-weight: 700;
    color: #fff;
    margin: 0 0 0.25rem;
  }

  .page-sub {
    font-size: 0.875rem;
    color: rgba(255, 255, 255, 0.8);
    margin: 0;
  }

  .btn-band {
    background: #fff;
    color: var(--blue-900);
    border: 1px solid #fff;
    font-family: var(--font-display);
    font-size: 0.875rem;
    font-weight: 600;
    white-space: nowrap;
    border-radius: var(--radius-pill);
    padding: 0.5rem 1.25rem;
    cursor: pointer;
  }

  .btn-band:hover { background: var(--blue-50); }

  .btn-primary {
    background: var(--blue-900);
    color: white;
    border: 1px solid var(--blue-900);
    font-family: var(--font-display);
    border-radius: var(--radius-pill);
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

  /* Filtros */
  .filters {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    flex-wrap: wrap;
  }

  .filter-input, .filter-select {
    background: var(--white);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    padding: 0.5rem 0.75rem;
    font-size: 0.875rem;
    color: var(--text);
    outline: none;
    transition: border-color 0.15s;
  }

  .filter-input:focus, .filter-select:focus {
    border-color: var(--purple);
  }

  .filter-search { min-width: 220px; flex: 1; max-width: 320px; }
  .filter-select { cursor: pointer; }

  .btn-clear {
    background: none;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    padding: 0.5rem 0.75rem;
    font-size: 0.8rem;
    color: var(--muted);
    cursor: pointer;
    transition: all 0.15s;
    white-space: nowrap;
  }

  .btn-clear:hover {
    border-color: var(--purple);
    color: var(--purple);
  }

  .results-count {
    font-size: 0.8rem;
    color: var(--muted);
    margin: 0;
  }

  /* Formulario */
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
    color: #dc2626;
    background: #fef2f2;
    border: 1px solid #fca5a5;
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
    min-width: 220px;
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

  .teams-container {
    background: var(--white);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: 1.5rem;
    box-shadow: 0 1px 4px rgba(0,0,0,0.04);
  }
</style>