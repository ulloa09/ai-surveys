<script lang="ts">
  // Encabezado institucional: barra superior de contacto, wordmark a la
  // izquierda y navegación a la derecha, con la franja de acento
  // debajo.

  type NavItem = { href: string; label: string };

  let {
    navItems = [] as NavItem[],
    currentPath = '',
    user = null as { display_name?: string; role_label?: string } | null,
    onLogout = undefined as undefined | (() => void),
    homeHref = '/dashboard'
  } = $props();

  let menuOpen = $state(false);

  function isActive(href: string) {
    return currentPath === href || (href !== homeHref && currentPath.startsWith(href));
  }
</script>

<header class="header">
  <div class="topbar">
    <div class="topbar-inner">
      <a class="tel" href="mailto:support@example.com">support@example.com</a>
      <a class="account-link" href={homeHref}>Mi <strong>cuenta</strong></a>
    </div>
  </div>

  <div class="masthead">
    <div class="masthead-inner">
      <a class="brand" href={homeHref}>
        <span class="brand-mark">AI</span>
        <span class="brand-name">Surveys</span>
      </a>

      {#if navItems.length}
        <button
          class="menu-toggle"
          aria-expanded={menuOpen}
          aria-controls="nav-principal"
          onclick={() => (menuOpen = !menuOpen)}
        >
          <span class="bars" aria-hidden="true"></span>
          MENÚ
        </button>
      {/if}

      <nav class="nav" id="nav-principal" class:open={menuOpen}>
        {#each navItems as item}
          <a href={item.href} class="nav-link" class:active={isActive(item.href)}>
            {item.label}
          </a>
        {/each}

        {#if user}
          <div class="user">
            <span class="user-name">{user.display_name ?? 'Usuario'}</span>
            {#if user.role_label}
              <span class="user-role">{user.role_label}</span>
            {/if}
          </div>
          {#if onLogout}
            <button class="btn-logout" onclick={onLogout}>Salir</button>
          {/if}
        {/if}
      </nav>
    </div>
  </div>

  <div class="accent"></div>
</header>

<style>
  .header {
    position: sticky;
    top: 0;
    z-index: 100;
    background: var(--white);
  }

  /* ── Barra superior ─────────────────────────────────────────────────── */
  .topbar {
    background: var(--white);
    border-bottom: 1px solid var(--border);
  }

  .topbar-inner {
    max-width: var(--container);
    margin: 0 auto;
    padding: 0.4rem 2rem;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
  }

  .tel {
    font-family: var(--font-display);
    font-weight: 700;
    font-size: 0.78rem;
    color: var(--blue-900);
    text-decoration: none;
    letter-spacing: 0.01em;
  }

  .account-link {
    font-family: var(--font-display);
    font-size: 0.78rem;
    font-weight: 400;
    color: var(--white);
    background: var(--blue-900);
    border-radius: var(--radius-pill);
    padding: 0.3rem 1.1rem;
    text-decoration: none;
  }

  .account-link strong { font-weight: 700; }
  .account-link:hover { background: var(--blue-700); }

  /* ── Logotipo y navegación ──────────────────────────────────────────── */
  .masthead-inner {
    max-width: var(--container);
    margin: 0 auto;
    padding: 0.85rem 2rem;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1.5rem;
  }

  .brand {
    display: flex;
    align-items: baseline;
    gap: 0.3rem;
    flex-shrink: 0;
    text-decoration: none;
    font-family: var(--font-display);
  }

  .brand-mark {
    font-weight: 700;
    font-size: 1.35rem;
    color: var(--blue-900);
  }

  .brand-name {
    font-weight: 500;
    font-size: 1.1rem;
    color: var(--blue-700);
  }

  .nav {
    display: flex;
    align-items: center;
    gap: 0.35rem;
  }

  .nav-link {
    font-family: var(--font-display);
    font-size: 0.82rem;
    font-weight: 600;
    color: var(--blue-900);
    text-decoration: none;
    padding: 0.55rem 0.8rem;
    border-bottom: 3px solid transparent;
    white-space: nowrap;
  }

  .nav-link:hover { color: var(--blue-500); }

  .nav-link.active {
    color: var(--blue-900);
    border-bottom-color: var(--blue-400);
  }

  .user {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    line-height: 1.25;
    margin-left: 1rem;
    padding-left: 1rem;
    border-left: 1px solid var(--border);
  }

  .user-name {
    font-size: 0.8rem;
    font-weight: 600;
    color: var(--text);
    white-space: nowrap;
  }

  .user-role {
    font-size: 0.68rem;
    color: var(--muted);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .btn-logout {
    margin-left: 0.9rem;
    background: transparent;
    border: 1px solid var(--blue-900);
    color: var(--blue-900);
    font-family: var(--font-display);
    font-size: 0.75rem;
    font-weight: 600;
    padding: 0.4rem 1rem;
    border-radius: var(--radius-pill);
    cursor: pointer;
  }

  .btn-logout:hover {
    background: var(--blue-900);
    color: var(--white);
  }

  /* ── Franja azul institucional ──────────────────────────────────────── */
  .accent {
    height: 5px;
    background: var(--blue-900);
  }

  /* ── Menú compacto ──────────────────────────────────────────────────── */
  .menu-toggle {
    display: none;
    align-items: center;
    gap: 0.6rem;
    background: transparent;
    border: none;
    font-family: var(--font-display);
    font-size: 0.82rem;
    font-weight: 700;
    letter-spacing: 0.04em;
    color: var(--blue-900);
    cursor: pointer;
    padding: 0.5rem;
  }

  .bars,
  .bars::before,
  .bars::after {
    display: block;
    width: 22px;
    height: 2px;
    background: var(--blue-900);
    content: '';
  }

  .bars { position: relative; }
  .bars::before { position: absolute; top: -7px; }
  .bars::after { position: absolute; top: 7px; }

  @media (max-width: 900px) {
    .topbar-inner,
    .masthead-inner { padding-left: 1.25rem; padding-right: 1.25rem; }

    .brand-mark { font-size: 1.15rem; }
    .brand-name { font-size: 0.95rem; }

    .menu-toggle { display: inline-flex; }

    .nav {
      display: none;
      position: absolute;
      left: 0;
      right: 0;
      top: 100%;
      flex-direction: column;
      align-items: stretch;
      gap: 0;
      background: var(--white);
      border-top: 1px solid var(--border);
      box-shadow: var(--shadow-md);
      padding: 0.5rem 1.25rem 1rem;
    }

    .nav.open { display: flex; }

    .nav-link {
      padding: 0.8rem 0;
      border-bottom: 1px solid var(--border);
    }

    .nav-link.active {
      border-bottom-color: var(--blue-400);
    }

    .user {
      align-items: flex-start;
      margin: 0.9rem 0 0;
      padding: 0;
      border-left: none;
    }

    .btn-logout {
      margin: 0.9rem 0 0;
      align-self: flex-start;
    }
  }

  .masthead { position: relative; }
</style>
