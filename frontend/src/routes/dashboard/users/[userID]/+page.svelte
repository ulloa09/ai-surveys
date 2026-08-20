<script lang="ts">
	import { enhance } from '$app/forms';
	import type { PageData } from './$types';
	import { ROLE_LABELS } from '$lib/types';

	let { data }: { data: PageData } = $props();

	const targetUser = $derived(data.targetUser);
	const isSelf = $derived(data.user?.id === targetUser?.id);

	let savingProfile = $state(false);
	let profileError = $state('');
	let profileSuccess = $state(false);

	let savingRole = $state(false);
	let roleError = $state('');
	let roleSuccess = $state(false);

	let savingPassword = $state(false);
	let passwordError = $state('');
	let passwordSuccess = $state(false);
	let newPassword = $state('');

	let deleting = $state(false);
	let deleteError = $state('');
	let confirmingDelete = $state(false);
</script>

<svelte:head><title>{targetUser?.display_name ?? 'Usuario'} — AI Surveys</title></svelte:head>

<div class="page">
	<div class="page-header">
		<div class="header-left">
			<a href="/dashboard/users" class="back-link">← Usuarios</a>
			<h1 class="page-title">{targetUser?.display_name}</h1>
			<p class="page-sub">
				<span class="role-badge role-{targetUser?.role}"
					>{ROLE_LABELS[targetUser?.role as keyof typeof ROLE_LABELS]}</span
				>
				{#if isSelf}<span class="self-tag">Esta es tu cuenta</span>{/if}
			</p>
		</div>
	</div>

	<!-- Editar perfil -->
	<div class="card">
		<h2 class="card-title">Perfil</h2>
		{#if profileError}<p class="form-error">{profileError}</p>{/if}
		{#if profileSuccess}<p class="form-success">Perfil actualizado.</p>{/if}
		<form
			method="POST"
			action="?/updateUser"
			use:enhance={() => {
				savingProfile = true;
				profileError = '';
				profileSuccess = false;
				return async ({ result, update }) => {
					savingProfile = false;
					if (result.type === 'failure') {
						profileError = (result.data as { updateError?: string })?.updateError ?? 'No se pudo actualizar';
					} else {
						profileSuccess = true;
						await update();
					}
				};
			}}
			class="form-fields"
		>
			<div class="field">
				<label for="edit-name">Nombre</label>
				<input
					id="edit-name"
					name="display_name"
					type="text"
					value={targetUser?.display_name}
					required
					disabled={savingProfile}
				/>
			</div>
			<div class="field">
				<label for="edit-email">Correo</label>
				<input
					id="edit-email"
					name="email"
					type="email"
					value={targetUser?.email}
					required
					disabled={savingProfile}
				/>
			</div>
			<button type="submit" class="btn-primary" disabled={savingProfile}>
				{savingProfile ? 'Guardando…' : 'Guardar cambios'}
			</button>
		</form>
	</div>

	<!-- Cambiar rol -->
	<div class="card">
		<h2 class="card-title">Rol</h2>
		{#if isSelf}
			<p class="hint">No puedes cambiar tu propio rol. Pide a otro Super Admin que lo haga.</p>
		{:else}
			{#if roleError}<p class="form-error">{roleError}</p>{/if}
			{#if roleSuccess}<p class="form-success">Rol actualizado.</p>{/if}
			<form
				method="POST"
				action="?/changeRole"
				use:enhance={() => {
					savingRole = true;
					roleError = '';
					roleSuccess = false;
					return async ({ result, update }) => {
						savingRole = false;
						if (result.type === 'failure') {
							roleError = (result.data as { roleError?: string })?.roleError ?? 'No se pudo cambiar el rol';
						} else {
							roleSuccess = true;
							await update();
						}
					};
				}}
				class="form-fields"
			>
				<div class="field field-sm">
					<label for="role-select">Nuevo rol</label>
					<select id="role-select" name="role" disabled={savingRole}>
						<option value="alumno" selected={targetUser?.role === 'alumno'}>Alumno</option>
						<option value="profesor" selected={targetUser?.role === 'profesor'}>Profesor</option>
						<option value="admin" selected={targetUser?.role === 'admin'}>Administrador</option>
						<option value="super_admin" selected={targetUser?.role === 'super_admin'}>Super Admin</option>
					</select>
				</div>
				<button type="submit" class="btn-primary" disabled={savingRole}>
					{savingRole ? 'Cambiando…' : 'Cambiar rol'}
				</button>
			</form>
		{/if}
	</div>

	<!-- Restablecer contraseña -->
	<div class="card">
		<h2 class="card-title">Restablecer contraseña</h2>
		<p class="hint">
			Se establece una contraseña nueva de inmediato y se cierran todas las sesiones activas del
			usuario.
		</p>
		{#if passwordError}<p class="form-error">{passwordError}</p>{/if}
		{#if passwordSuccess}<p class="form-success">Contraseña restablecida.</p>{/if}
		<form
			method="POST"
			action="?/resetPassword"
			use:enhance={() => {
				savingPassword = true;
				passwordError = '';
				passwordSuccess = false;
				return async ({ result, update }) => {
					savingPassword = false;
					if (result.type === 'failure') {
						passwordError = (result.data as { passwordError?: string })?.passwordError ?? 'No se pudo restablecer';
					} else {
						passwordSuccess = true;
						newPassword = '';
						await update();
					}
				};
			}}
			class="form-fields"
		>
			<div class="field">
				<label for="new-password">Nueva contraseña</label>
				<input
					id="new-password"
					name="new_password"
					type="password"
					placeholder="Mínimo 8 caracteres"
					minlength="8"
					required
					bind:value={newPassword}
					disabled={savingPassword}
				/>
			</div>
			<button type="submit" class="btn-primary" disabled={savingPassword}>
				{savingPassword ? 'Restableciendo…' : 'Restablecer contraseña'}
			</button>
		</form>
	</div>

	<!-- Eliminar usuario -->
	<div class="card card-danger">
		<h2 class="card-title">Eliminar usuario</h2>
		{#if isSelf}
			<p class="hint">No puedes eliminar tu propia cuenta.</p>
		{:else}
			<p class="hint">
				Esta acción no se puede deshacer. No se permite si el usuario es dueño de encuestas o creó
				equipos.
			</p>
			{#if deleteError}<p class="form-error">{deleteError}</p>{/if}
			{#if !confirmingDelete}
				<button class="btn-danger" onclick={() => (confirmingDelete = true)}>Eliminar usuario</button>
			{:else}
				<form
					method="POST"
					action="?/deleteUser"
					use:enhance={() => {
						deleting = true;
						deleteError = '';
						return async ({ result }) => {
							deleting = false;
							if (result.type === 'failure') {
								confirmingDelete = false;
								deleteError = (result.data as { deleteError?: string })?.deleteError ?? 'No se pudo eliminar';
							}
							// en éxito, la acción redirige del lado del servidor
						};
					}}
					class="confirm-row"
				>
					<span class="confirm-text">¿Confirmas eliminar a {targetUser?.display_name}?</span>
					<button type="submit" class="btn-danger" disabled={deleting}>
						{deleting ? 'Eliminando…' : 'Sí, eliminar'}
					</button>
					<button
						type="button"
						class="btn-cancel"
						onclick={() => (confirmingDelete = false)}
						disabled={deleting}
					>
						Cancelar
					</button>
				</form>
			{/if}
		{/if}
	</div>
</div>

<style>
	.page {
		display: flex;
		flex-direction: column;
		gap: 1.5rem;
		padding: 2rem;
		width: 100%;
		max-width: 720px;
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

	.back-link:hover {
		color: var(--purple);
	}

	.page-title {
		font-size: 1.5rem;
		font-weight: 700;
		color: var(--text);
		margin: 0;
	}

	.page-sub {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		margin: 0;
	}

	.self-tag {
		font-size: 0.75rem;
		color: var(--muted);
		font-style: italic;
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

	.card {
		background: var(--white);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		padding: 1.5rem;
		box-shadow: 0 1px 4px rgba(0, 0, 0, 0.04);
	}

	.card-danger {
		border-color: #fca5a5;
	}

	.card-title {
		font-size: 1rem;
		font-weight: 600;
		color: var(--text);
		margin: 0 0 1rem;
	}

	.hint {
		font-size: 0.8rem;
		color: var(--muted);
		margin: 0 0 1rem;
	}

	.form-error {
		font-size: 0.8rem;
		color: #991b1b;
		background: #fef2f2;
		border: 1px solid #fca5a5;
		border-radius: var(--radius-sm);
		padding: 0.5rem 0.75rem;
		margin-bottom: 1rem;
	}

	.form-success {
		font-size: 0.8rem;
		color: #166534;
		background: #f0fdf4;
		border: 1px solid #86efac;
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
		flex: 0 0 200px;
		min-width: 200px;
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

	.btn-danger {
		background: #dc2626;
		color: white;
		border: none;
		border-radius: var(--radius-sm);
		padding: 0.5rem 1.25rem;
		font-size: 0.875rem;
		font-weight: 600;
		cursor: pointer;
		transition: opacity 0.15s;
		white-space: nowrap;
	}

	.btn-danger:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
	.btn-danger:not(:disabled):hover {
		opacity: 0.85;
	}

	.btn-cancel {
		background: none;
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		padding: 0.5rem 1.25rem;
		font-size: 0.875rem;
		color: var(--muted);
		cursor: pointer;
	}

	.confirm-row {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		flex-wrap: wrap;
	}

	.confirm-text {
		font-size: 0.875rem;
		color: var(--text);
	}
</style>
