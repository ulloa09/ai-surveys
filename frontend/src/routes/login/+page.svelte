<script lang="ts">
  import { enhance } from '$app/forms';
  import type { ActionData, PageData } from './$types';

  let { data, form }: { data: PageData; form: ActionData } = $props();
</script>

<svelte:head>
  <title>AI Surveys — Iniciar sesión</title>
</svelte:head>

<div class="screen">
  <div class="card">
    <div class="card-head">
      <p class="wordmark"><span class="mark">AI</span> Surveys</p>
      <h1>Iniciar sesión</h1>
      <p class="sub">Plataforma de encuestas conversacionales</p>
    </div>

    {#if form?.error}
      <div class="error" role="alert">{form.error}</div>
    {/if}

    <form method="POST" action="?/login" use:enhance>
      <!-- action="?/login" no conserva el ?redirect= de esta página (ver
           +page.server.ts) — viaja como campo oculto del form en su lugar. -->
      <input type="hidden" name="redirect" value={data.redirect ?? ''} />
      <div class="field">
        <label for="l-email">Correo electrónico</label>
        <input
          id="l-email"
          type="email"
          name="email"
          value={form?.email ?? ''}
          required
          autocomplete="email"
          placeholder="correo@example.com"
        />
      </div>
      <div class="field">
        <label for="l-pass">Contraseña</label>
        <input
          id="l-pass"
          type="password"
          name="password"
          required
          autocomplete="current-password"
          placeholder="••••••••"
        />
      </div>
      <button type="submit" class="submit-btn">Iniciar sesión</button>
    </form>

    <p class="help">
      ¿Problemas para acceder? Escríbenos a
      <a href="mailto:support@example.com">support@example.com</a>
    </p>
  </div>
</div>

<style>
  .screen {
    position: fixed;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 1.5rem;
    padding: 1.5rem;
    overflow: auto;
    background: linear-gradient(160deg, var(--blue-900), var(--blue-700));
  }

  .card {
    background: var(--white);
    border-top: 5px solid var(--blue-900);
    border-radius: var(--radius-md);
    padding: 2.5rem 2.25rem 2rem;
    width: 100%;
    max-width: 420px;
    box-shadow: var(--shadow-lg);
  }

  .card-head {
    text-align: center;
    margin-bottom: 2rem;
  }

  .wordmark {
    margin: 0 0 1.25rem;
    font-family: var(--font-display);
    font-size: 1.1rem;
    font-weight: 500;
    color: var(--blue-700);
  }

  .wordmark .mark {
    font-weight: 700;
    color: var(--blue-900);
  }

  h1 {
    font-size: 1.4rem;
    color: var(--blue-900);
    margin: 0 0 0.25rem;
  }

  .sub {
    font-size: 0.85rem;
    color: var(--muted);
    margin: 0;
  }

  .field {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
    margin-bottom: 1rem;
  }

  label {
    font-family: var(--font-display);
    font-size: 0.775rem;
    font-weight: 600;
    color: var(--blue-900);
  }

  input {
    width: 100%;
    padding: 0.7rem 0.9rem;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    font-size: 0.9375rem;
    color: var(--text);
    background: var(--white);
    transition: border-color 0.15s;
  }

  input::placeholder { color: #9aa3ad; }

  input:focus {
    outline: none;
    border-color: var(--blue-900);
    box-shadow: inset 0 0 0 1px var(--blue-900);
  }

  .submit-btn {
    width: 100%;
    padding: 0.8rem;
    margin-top: 0.75rem;
    border: 1px solid var(--blue-900);
    border-radius: var(--radius-pill);
    background: var(--blue-900);
    color: #fff;
    font-family: var(--font-display);
    font-weight: 600;
    font-size: 0.9375rem;
    cursor: pointer;
    transition: background 0.15s;
  }

  .submit-btn:hover { background: var(--blue-700); border-color: var(--blue-700); }

  .help {
    margin: 1.5rem 0 0;
    padding-top: 1.25rem;
    border-top: 1px solid var(--border);
    font-size: 0.78rem;
    color: var(--muted);
    text-align: center;
  }

  .help a {
    color: var(--blue-600);
    font-weight: 600;
    text-decoration: none;
  }

  .error {
    background: var(--danger-lt);
    border-left: 3px solid var(--danger);
    color: var(--danger);
    padding: 0.7rem 0.9rem;
    border-radius: var(--radius-sm);
    margin-bottom: 1.25rem;
    font-size: 0.85rem;
  }
</style>
