<script lang="ts">
	import { enhance } from '$app/forms';
	import { goto } from '$app/navigation';
	import type { PageData } from './$types';
	import { ROLE_LABELS, type User } from '$lib/types';

	let { data }: { data: PageData } = $props();

	let showCreateForm = $state(false);
	let creating = $state(false);
	let createError = $state('');

	let search = $state(data.search ?? '');
	$effect(() => {
		search = data.search ?? '';
	});
	let role = $state(data.role ?? '');

	let searchTimeout: ReturnType<typeof setTimeout>;

	function applyFilters() {
		const url = new URL(window.location.href);
		if (search) {
			url.searchParams.set('search', search);
		} else {
			url.searchParams.delete('search');
		}
		if (role) {
			url.searchParams.set('role', role);
		} else {
			url.searchParams.delete('role');
		}
		goto(url.toString(), { replaceState: true, noScroll: true, keepFocus: true });
	}

	function handleSearch() {
		clearTimeout(searchTimeout);
		searchTimeout = setTimeout(applyFilters, 500);
	}

	function handleFilterChange() {
		applyFilters();
	}

	function clearFilters() {
		search = '';
		role = '';
		goto('/dashboard/users', { replaceState: true, noScroll: true });
	}

	const hasActiveFilters = $derived(Boolean(search || role));
	const users = $derived((data.users ?? []) as User[]);
</script>

<svelte:head><title>Usuarios — AI Surveys</title></svelte:head>

<div class="page">
	<!-- Banda institucional de título -->
	<div class="page-header">
		<div class="page-header-inner">
			<div>
				<h1 class="page-title">Usuarios</h1>
				<p class="page-sub">Gestión de cuentas de la plataforma — exclusivo de Super Admin</p>
			</div>
			<button class="btn-band" onclick={() => (showCreateForm = !showCreateForm)}>
				{showCreateForm ? 'Cancelar' : '+ Nuevo usuario'}
			</button>
		</div>
	</div>

	<div class="page-content">
		<div class="filters">
			<input
				type="text"
				placeholder="Buscar por nombre o correo…"
				bind:value={search}
				oninput={handleSearch}
				class="filter-input filter-search"
			/>

			<select bind:value={role} onchange={handleFilterChange} class="filter-select">
				<option value="">Todos los roles</option>
				<option value="super_admin">Super Admin</option>
				<option value="admin">Administrador</option>
				<option value="profesor">Profesor</option>
				<option value="alumno">Alumno</option>
			</select>

			{#if hasActiveFilters}
				<button class="btn-clear" onclick={clearFilters}>✕ Limpiar filtros</button>
			{/if}
		</div>

		{#if hasActiveFilters}
			<p class="results-count">{users.length} usuario(s) encontrado(s)</p>
		{/if}

		{#if showCreateForm}
			<div class="form-card">
				<h2 class="form-title">Crear usuario</h2>
				{#if createError}
					<p class="form-error">{createError}</p>
				{/if}
				<form
					method="POST"
					action="?/createUser"
					use:enhance={() => {
						creating = true;
						createError = '';
						return async ({ result, update }) => {
							creating = false;
							if (result.type === 'success') {
								showCreateForm = false;
								await update();
							} else if (result.type === 'failure') {
								createError = (result.data as { error?: string })?.error ?? 'Error al crear usuario';
							}
						};
					}}
					class="form-fields"
				>
					<div class="field">
						<label for="user-name">Nombre</label>
						<input
							id="user-name"
							name="display_name"
							type="text"
							placeholder="Nombre completo"
							required
							disabled={creating}
						/>
					</div>
					<div class="field">
						<label for="user-email">Correo</label>
						<input
							id="user-email"
							name="email"
							type="email"
							placeholder="correo@example.com"
							required
							disabled={creating}
						/>
					</div>
					<div class="field">
						<label for="user-password">Contraseña</label>
						<input
							id="user-password"
							name="password"
							type="password"
							placeholder="Mínimo 8 caracteres"
							required
							minlength="8"
							disabled={creating}
						/>
					</div>
					<div class="field field-sm">
						<label for="user-role">Rol</label>
						<select id="user-role" name="role" disabled={creating}>
							<option value="alumno">Alumno</option>
							<option value="profesor">Profesor</option>
							<option value="admin">Administrador</option>
							<option value="super_admin">Super Admin</option>
						</select>
					</div>
					<button type="submit" class="btn-primary" disabled={creating}>
						{creating ? 'Creando…' : 'Crear usuario'}
					</button>
				</form>
			</div>
		{/if}

		<div class="users-container">
			{#if users.length === 0}
				<p class="empty-state">No hay usuarios que coincidan con los filtros.</p>
			{:else}
				<table class="users-table">
					<thead>
						<tr>
							<th>Nombre</th>
							<th>Correo</th>
							<th>Rol</th>
							<th>Creado</th>
						</tr>
					</thead>
					<tbody>
						{#each users as u (u.id)}
							<tr class="user-row" onclick={() => goto(`/dashboard/users/${u.id}`)}>
								<td>{u.display_name}</td>
								<td>{u.email}</td>
								<td><span class="role-badge role-{u.role}">{ROLE_LABELS[u.role]}</span></td>
								<td>{new Date(u.created_at).toLocaleDateString('es-MX')}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			{/if}
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
		border-radius: var(--radius-pill);
		padding: 0.5rem 1.25rem;
		font-family: var(--font-display);
		font-size: 0.875rem;
		font-weight: 600;
		cursor: pointer;
		white-space: nowrap;
		flex-shrink: 0;
	}

	.btn-band:hover {
		background: var(--blue-50);
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

	.btn-primary:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
	.btn-primary:not(:disabled):hover {
		opacity: 0.85;
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
		border-color: var(--purple);
	}

	.filter-search {
		min-width: 220px;
		flex: 1;
		max-width: 320px;
	}
	.filter-select {
		cursor: pointer;
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

	.form-card {
		background: var(--white);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		padding: 1.5rem;
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
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
		min-width: 200px;
	}

	.field-sm {
		flex: 0 0 160px;
		min-width: 160px;
	}

	label {
		font-size: 0.75rem;
		font-weight: 600;
		color: var(--muted);
		text-transform: uppercase;
		letter-spacing: 0.5px;
	}

	input,
	select {
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

	input:focus,
	select:focus {
		border-color: var(--purple);
	}

	.users-container {
		background: var(--white);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		padding: 1.5rem;
		box-shadow: 0 1px 4px rgba(0, 0, 0, 0.04);
	}

	.empty-state {
		text-align: center;
		color: var(--muted);
		font-size: 0.875rem;
		padding: 1.5rem 0;
		margin: 0;
	}

	.users-table {
		width: 100%;
		border-collapse: collapse;
	}

	.users-table th {
		text-align: left;
		font-size: 0.75rem;
		font-weight: 600;
		color: var(--muted);
		text-transform: uppercase;
		letter-spacing: 0.5px;
		padding: 0.5rem 0.75rem;
		border-bottom: 1px solid var(--border);
	}

	.users-table td {
		padding: 0.65rem 0.75rem;
		font-size: 0.875rem;
		color: var(--text);
		border-bottom: 1px solid var(--border);
	}

	.user-row {
		cursor: pointer;
		transition: background 0.15s;
	}

	.user-row:hover {
		background: var(--bg);
	}

	.role-badge {
		display: inline-block;
		font-size: 0.7rem;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		padding: 0.15rem 0.55rem;
		border-radius: var(--radius-pill);
	}

	.role-super_admin {
		background: var(--blue-900);
		color: #fff;
	}
	.role-admin {
		background: var(--blue-100);
		color: var(--blue-900);
	}
	.role-profesor {
		background: var(--blue-50);
		color: var(--blue-600);
	}
	.role-alumno {
		background: #f1f2f4;
		color: var(--muted);
	}
</style>
