<script lang="ts">
	import { enhance } from '$app/forms';
	import { getPermissions } from '$lib/permissions';
	import { STATUS_LABELS } from '$lib/types';
	import type { Survey } from '$lib/types';
	import SurveyList from '$lib/components/surveys/SurveyList.svelte';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	const perms = $derived(getPermissions(data.user?.role ?? 'alumno'));

	let actionError = $state('');
	let confirmDeleteID = $state('');
	let pendingDeleteID = $state('');
	let pendingDupID = $state('');

	// Filtro por estado y búsqueda por título (#16). Se resuelven en el cliente:
	// el listado ya viene completo del servidor, así que filtrar aquí evita un
	// round-trip por cada tecla. "Mostrar archivadas" sí es server-side, porque
	// el backend las excluye de la query salvo que se le pidan.
	let statusFilter = $state<'all' | Survey['status']>('all');
	let searchQuery = $state('');

	// 'archived' solo se ofrece cuando el toggle las trajo: filtrar por un estado
	// que el servidor omitió siempre daría cero resultados.
	const statusOptions = $derived.by(() => {
		const all = Object.keys(STATUS_LABELS) as Survey['status'][];
		return data.showArchived ? all : all.filter((status) => status !== 'archived');
	});

	const surveys = $derived(data.surveys ?? []);

	const filteredSurveys = $derived.by(() => {
		const needle = searchQuery.trim().toLowerCase();
		return surveys.filter((survey) => {
			if (statusFilter !== 'all' && survey.status !== statusFilter) return false;
			if (needle && !survey.title.toLowerCase().includes(needle)) return false;
			return true;
		});
	});

	// Distinguir "no hay encuestas" de "el filtro no encontró nada".
	const filteredOut = $derived(surveys.length > 0 && filteredSurveys.length === 0);

	function clearFilters() {
		statusFilter = 'all';
		searchQuery = '';
	}

	function requestDelete(id: string) {
		confirmDeleteID = id;
	}
	function cancelDelete() {
		confirmDeleteID = '';
	}

	function requestDuplicate(id: string) {
		pendingDupID = id;
		setTimeout(() => {
			document
				.getElementById('form-dup')
				?.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
		}, 0);
	}
</script>

<svelte:head><title>Encuestas — AI Surveys</title></svelte:head>

<!-- Hidden forms -->
<form
	id="form-dup"
	method="POST"
	action="?/duplicate"
	style="display:none"
	use:enhance={() => async ({ result, update }) => {
		pendingDupID = '';
		if (result.type === 'failure') actionError = (result.data as any)?.error ?? 'Error al duplicar';
		else {
			actionError = '';
			await update();
		}
	}}
>
	<input type="hidden" name="id" value={pendingDupID} />
</form>

<form
	id="form-del"
	method="POST"
	action="?/delete"
	style="display:none"
	use:enhance={() => async ({ result, update }) => {
		pendingDeleteID = '';
		confirmDeleteID = '';
		if (result.type === 'failure') actionError = (result.data as any)?.error ?? 'Error al eliminar';
		else {
			actionError = '';
			await update();
		}
	}}
>
	<input type="hidden" name="id" value={pendingDeleteID} />
</form>

<div class="page">
	<!-- Banda institucional de título -->
	<div class="page-header">
		<div class="page-header-inner">
			<div>
				<h1 class="page-title">
					{data.user?.role === 'super_admin' ? 'Todas las encuestas' : 'Mis encuestas'}
				</h1>
				<p class="page-sub">
					{data.user?.role === 'super_admin'
						? 'Vista global de todas las encuestas de la plataforma'
						: 'Las encuestas de tu equipo'}
				</p>
			</div>
			<div class="header-actions">
				<a
					class="archive-toggle"
					href={data.showArchived ? '/dashboard/surveys' : '/dashboard/surveys?archived=true'}
					data-sveltekit-noscroll
				>
					<input type="checkbox" checked={data.showArchived} tabindex="-1" readonly />
					Mostrar archivadas
				</a>
				{#if perms.canCreateSurvey}
					<a href="/admin/surveys" class="btn-band">+ Nueva encuesta</a>
				{/if}
			</div>
		</div>
	</div>

	<div class="page-content">
		{#if actionError}
			<div class="action-error">
				{actionError}
				<button onclick={() => (actionError = '')}>✕</button>
			</div>
		{/if}

		<!-- Modal delete -->
		{#if confirmDeleteID}
			<div class="confirm-overlay">
				<div class="confirm-card">
					<h3>¿Eliminar esta encuesta?</h3>
					<p>
						Esta acción no se puede deshacer. Solo se pueden eliminar encuestas en borrador sin
						respuestas.
					</p>
					<div class="confirm-actions">
						<button class="btn-cancel" onclick={cancelDelete}>Cancelar</button>
						<button
							class="btn-danger"
							onclick={() => {
								pendingDeleteID = confirmDeleteID;
								setTimeout(() => {
									document
										.getElementById('form-del')
										?.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
								}, 0);
							}}
						>
							Sí, eliminar
						</button>
					</div>
				</div>
			</div>
		{/if}

		{#if surveys.length > 0}
			<div class="filters">
				<input
					type="text"
					placeholder="Buscar por título…"
					aria-label="Buscar encuestas por título"
					bind:value={searchQuery}
					class="filter-input filter-search"
				/>
				<select bind:value={statusFilter} aria-label="Filtrar por estado" class="filter-select">
					<option value="all">Todos los estados</option>
					{#each statusOptions as status (status)}
						<option value={status}>{STATUS_LABELS[status]}</option>
					{/each}
				</select>
				<span class="results-count">{filteredSurveys.length} de {surveys.length}</span>
			</div>
		{/if}

		{#if filteredOut}
			<div class="no-matches">
				<p>Ninguna encuesta coincide con el filtro.</p>
				<button class="btn-clear" onclick={clearFilters}>Limpiar filtros</button>
			</div>
		{:else}
			<SurveyList
				surveys={filteredSurveys}
				canEdit={perms.canEditSurvey}
				canDelete={perms.canDeleteSurvey}
				canDuplicate={perms.canDuplicateSurvey}
				onDelete={requestDelete}
				onDuplicate={requestDuplicate}
			/>
		{/if}
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

	.header-actions {
		display: flex;
		align-items: center;
		gap: 1rem;
		flex-shrink: 0;
	}

	.btn-band {
		background: #fff;
		color: var(--blue-900);
		border: 1px solid #fff;
		border-radius: var(--radius-pill);
		padding: 0.5rem 1.25rem;
		font-family: var(--font-display);
		font-size: 0.875rem;
		font-weight: 600;
		cursor: pointer;
		white-space: nowrap;
		flex-shrink: 0;
		text-decoration: none;
		display: inline-block;
	}

	.btn-band:hover {
		background: var(--blue-50);
	}

	.archive-toggle {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		font-size: 0.85rem;
		color: rgba(255, 255, 255, 0.75);
		text-decoration: none;
		cursor: pointer;
		white-space: nowrap;
		transition: color 0.15s;
	}

	.archive-toggle:hover {
		color: white;
	}
	.archive-toggle input {
		accent-color: var(--blue-400);
		pointer-events: none;
	}

	.filters {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		flex-wrap: wrap;
	}

	.filter-input,
	.filter-select {
		background: var(--white);
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		padding: 0.5rem 0.75rem;
		font-size: 0.875rem;
		color: var(--text);
		outline: none;
		transition: border-color 0.15s;
	}

	.filter-input:focus,
	.filter-select:focus {
		border-color: var(--blue-900);
	}

	.filter-search {
		min-width: 220px;
		flex: 1;
		max-width: 320px;
	}
	.filter-select {
		cursor: pointer;
	}

	.results-count {
		font-size: 0.8rem;
		color: var(--muted);
		margin-left: auto;
	}

	.no-matches {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 1rem;
		padding: 2.5rem 1rem;
		text-align: center;
		background: var(--white);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
	}

	.no-matches p {
		font-size: 0.9rem;
		color: var(--muted);
		margin: 0;
	}

	.btn-clear {
		background: none;
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		padding: 0.5rem 0.75rem;
		font-size: 0.8rem;
		color: var(--muted);
		cursor: pointer;
		transition: all 0.15s;
	}

	.btn-clear:hover {
		border-color: var(--blue-900);
		color: var(--blue-900);
	}

	.action-error {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		background: var(--danger-lt);
		border-left: 3px solid var(--danger);
		border-radius: var(--radius-sm);
		padding: 0.65rem 1rem;
		font-size: 0.875rem;
		color: var(--danger);
	}

	.action-error button {
		background: none;
		border: none;
		color: var(--danger);
		cursor: pointer;
		font-size: 1rem;
		padding: 0;
	}

	.confirm-overlay {
		position: fixed;
		inset: 0;
		background: rgba(15, 23, 42, 0.45);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 200;
		backdrop-filter: blur(4px);
	}

	.confirm-card {
		background: var(--white);
		border-top: 5px solid var(--blue-900);
		border-radius: var(--radius-md);
		padding: 2.25rem;
		max-width: 400px;
		width: 90%;
		box-shadow: var(--shadow-lg);
	}

	.confirm-card h3 {
		font-size: 1.05rem;
		font-weight: 700;
		color: var(--text);
		margin: 0 0 0.5rem;
		letter-spacing: -0.01em;
	}
	.confirm-card p {
		font-size: 0.875rem;
		color: var(--muted);
		margin: 0 0 1.75rem;
		line-height: 1.6;
	}

	.confirm-actions {
		display: flex;
		justify-content: flex-end;
		gap: 0.75rem;
	}

	.btn-cancel {
		background: none;
		border: 1px solid var(--border);
		border-radius: var(--radius-pill);
		padding: 0.5rem 1.125rem;
		font-size: 0.875rem;
		color: var(--muted);
		cursor: pointer;
		transition: all 0.15s;
	}

	.btn-cancel:hover {
		border-color: var(--blue-900);
		color: var(--blue-900);
	}

	.btn-danger {
		background: var(--danger);
		color: white;
		border: 1px solid var(--danger);
		border-radius: var(--radius-pill);
		padding: 0.5rem 1.125rem;
		font-size: 0.875rem;
		font-weight: 600;
		cursor: pointer;
		transition: opacity 0.15s;
	}

	.btn-danger:hover {
		opacity: 0.88;
	}

	@media (max-width: 700px) {
		.page-header {
			padding: 1.75rem 1.5rem;
		}
		.page-content {
			padding: 1.5rem;
		}
		.page-header-inner {
			flex-direction: column;
			align-items: flex-start;
		}
		.header-actions {
			width: 100%;
			justify-content: flex-end;
		}
	}
</style>
