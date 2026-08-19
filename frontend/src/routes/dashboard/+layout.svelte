<script lang="ts">
  import { page } from '$app/stores';
  import type { LayoutData } from './$types';
  import { getPermissions } from '$lib/permissions';
  import { ROLE_LABELS } from '$lib/types';
  import InstitutionalHeader from '$lib/components/shared/InstitutionalHeader.svelte';
  import InstitutionalFooter from '$lib/components/shared/InstitutionalFooter.svelte';

  let { data, children }: { data: LayoutData; children: any } = $props();

  const role  = $derived(data.user?.role ?? 'alumno');
  const perms = $derived(getPermissions(role));

  // Alumno: ABSOLUTAMENTE sin acceso a dashboards/analytics/reportes (Fase 3).
  // El backend ya rechaza estas rutas para alumno, pero bloqueamos aquí
  // también para que ni el shell ni el contenido de la página lleguen a
  // montarse — defensa en profundidad, sin depender solo del 403 del API.
  const isAlumno = $derived(role === 'alumno');

  const navItems = $derived.by(() => {
    const items: { href: string; label: string }[] = [
      { href: '/dashboard', label: 'Inicio' },
    ];
    if (perms.canViewSurveys) {
      items.push({
        href: '/dashboard/surveys',
        label: role === 'super_admin' ? 'Todas las encuestas' : 'Mis encuestas'
      });
    }
    if (perms.canViewTeams) {
      items.push({ href: '/dashboard/teams', label: 'Equipos' });
    }
    if (perms.canViewResults) {
      items.push({ href: '/dashboard/analytics', label: 'Analytics' });
    }
    if (perms.canManageUsers) {
      items.push({ href: '/dashboard/users', label: 'Usuarios' });
    }
    if (role === 'super_admin') {
      items.push({ href: '/dashboard/settings', label: 'Configuración IA' });
    }
    return items;
  });

  const headerUser = $derived({
    display_name: data.user?.display_name ?? 'Usuario',
    role_label: ROLE_LABELS[role as keyof typeof ROLE_LABELS] ?? 'Usuario'
  });

  async function logout() {
    await fetch('/api/logout', { method: 'POST' });
    window.location.href = '/login';
  }
</script>

{#if isAlumno}
  <!-- Alumno no tiene ningún panel de administración — solo un mensaje y salir. -->
  <div class="no-access">
    <div class="no-access-card">
      <p class="no-access-wordmark"><span class="mark">AI</span> Surveys</p>
      <h1>Sin acceso a este panel</h1>
      <p>Tu cuenta de alumno no tiene un panel de administración. Ingresa a tu encuesta desde el enlace que te compartieron.</p>
      <button class="btn-logout" onclick={logout}>Salir</button>
    </div>
  </div>
{:else}
  <div class="shell">
    <InstitutionalHeader
      {navItems}
      currentPath={$page.url.pathname}
      user={headerUser}
      onLogout={logout}
      homeHref="/dashboard"
    />

    <main class="content">
      {@render children()}
    </main>

    <InstitutionalFooter />
  </div>
{/if}

<style>
  .no-access {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 100vh;
    padding: 1.5rem;
    background: var(--bg);
  }

  .no-access-card {
    background: var(--white);
    border: 1px solid var(--border);
    border-top: 5px solid var(--blue-900);
    border-radius: var(--radius-md);
    padding: 2.5rem;
    max-width: 420px;
    text-align: center;
    box-shadow: var(--shadow-md);
  }

  .no-access-wordmark {
    font-family: var(--font-display);
    font-size: 1.15rem;
    font-weight: 500;
    color: var(--blue-700);
    margin: 0 0 1.75rem;
  }

  .no-access-wordmark .mark {
    font-weight: 700;
    color: var(--blue-900);
  }

  .no-access-card h1 {
    font-size: 1.25rem;
    font-weight: 700;
    color: var(--blue-900);
    margin: 0 0 0.75rem;
  }

  .no-access-card p {
    font-size: 0.9rem;
    color: var(--muted);
    margin: 0 0 1.5rem;
  }

  .btn-logout {
    background: var(--blue-900);
    border: 1px solid var(--blue-900);
    border-radius: var(--radius-pill);
    color: var(--white);
    font-family: var(--font-display);
    font-size: 0.8rem;
    font-weight: 600;
    padding: 0.5rem 1.5rem;
    cursor: pointer;
  }

  .btn-logout:hover { background: var(--blue-700); }

  .shell {
    display: flex;
    flex-direction: column;
    min-height: 100vh;
    width: 100%;
  }

  .content {
    flex: 1;
    background: var(--bg);
    width: 100%;
  }
</style>
