<script lang="ts">
  import { enhance } from '$app/forms';
  import type { PageData, ActionData } from './$types';

  let { data, form }: { data: PageData; form: ActionData } = $props();

  let saving     = $state(false);
  let testing    = $state(false);
  let testResult = $state<{ ok: boolean; message: string } | null>(null);
  let showKey    = $state(false);

  let provider    = $state(data.settings?.active_provider ?? 'claude');
  let apiKey      = $state('');
  let claudeModel = $state(data.settings?.claude_model ?? 'claude-sonnet-4-6');
  let openaiModel = $state(data.settings?.openai_model ?? 'gpt-4o');

  const claudeModels = [
    'claude-opus-4-5',
    'claude-sonnet-4-5',
    'claude-haiku-4-5-20251001',
    ];
  const openaiModels = ['gpt-4o', 'gpt-4o-mini', 'gpt-4-turbo', 'gpt-5.4-mini'];

  const maskedKey = $derived(
    provider === 'claude'
      ? (data.settings?.claude_key_masked || '')
      : (data.settings?.openai_key_masked || '')
  );

  const modelOptions = $derived(provider === 'claude' ? claudeModels : openaiModels);
  const selectedModel = $derived(provider === 'claude' ? claudeModel : openaiModel);
  async function testConnection() {
    testing = true;
    testResult = null;
    try {
        const res = await fetch('/api/admin/settings/test', {
        method: 'POST',
        credentials: 'include'
        });
        const json = await res.json();
        testResult = {
        ok: json.ok === true,
        message: json.ok
            ? `Conexión exitosa con ${json.model}`
            : (json.error ?? 'Error desconocido')
        };
    } catch {
        testResult = { ok: false, message: 'No se pudo conectar al servidor' };
    } finally {
        testing = false;
    }
    }
</script>

<svelte:head><title>Configuración de IA — AI Surveys</title></svelte:head>

<div class="page">

  <!-- Banda institucional de título -->
  <div class="page-header">
    <div class="page-header-inner">
      <h1 class="page-title">Configuración de IA</h1>
      <p class="page-sub">Elige qué modelo conduce las entrevistas, configura tu API key y prueba la conexión antes de activar encuestas.</p>
    </div>
  </div>

  <div class="page-content">

  {#if form?.error}
    <div class="banner banner-error">{form.error}</div>
  {/if}
  {#if form?.success}
    <div class="banner banner-ok">Configuración guardada correctamente.</div>
  {/if}

  <form method="POST" action="?/save"
    use:enhance={() => {
        saving = true;
        return async ({ result, update }) => {
            saving = false;
            await update({ reset: false }); // ← reset:false evita que el form se limpie
        };
        }}
    class="layout"
  >
    <!-- Columna izquierda: selector de proveedor -->
    <div class="col-left">
      <div class="card">
        <h2 class="card-title">Proveedor activo</h2>

        <div class="provider-list">
          {#each [
            { value: 'claude', name: 'Claude (Anthropic)', desc: 'Claude entiende matices del español académico y mantiene conversaciones naturales. Ideal para retroalimentación estudiantil.' },
            { value: 'openai', name: 'OpenAI', desc: 'Destaca en seguimiento conversacional, extracción de información y análisis estructurado. Ideal para encuestas adaptativas y generación de insights.' }
          ] as opt}
            <label class="provider-row" class:selected={provider === opt.value}>
              <input type="radio" name="active_provider" value={opt.value}
                bind:group={provider} />
              <div class="provider-info">
                <strong>{opt.name}</strong>
                <span>{opt.desc}</span>
              </div>
            </label>
          {/each}
        </div>
      </div>
    </div>

    <!-- Columna derecha: configuración del proveedor seleccionado -->
    <div class="col-right">
      <div class="card">
        <h2 class="card-title">
          {provider === 'claude' ? 'Claude (Anthropic)' : 'OpenAI'}
        </h2>

        <div class="field">
          <label for="api-key">API Key</label>
          <div class="key-row">
            <input
              id="api-key"
              name={provider === 'claude' ? 'claude_key' : 'openai_key'}
              type={showKey ? 'text' : 'password'}
              bind:value={apiKey}
              placeholder={maskedKey || 'No configurada'}
            />
            <button type="button" class="btn-toggle"
              onclick={() => showKey = !showKey}>
              {showKey ? 'Ocultar' : 'Mostrar'}
            </button>
          </div>
          <p class="hint">Deja vacío para conservar la key actual.</p>
        </div>

        <div class="field">
          <label for="model-select">Modelo</label>
          <select id="model-select"
            name={provider === 'claude' ? 'claude_model' : 'openai_model'}
            value={selectedModel}
            onchange={(e) => {
              if (provider === 'claude') claudeModel = e.currentTarget.value;
              else openaiModel = e.currentTarget.value;
            }}
          >
            {#each modelOptions as m}
              <option value={m}>{m}</option>
            {/each}
          </select>
        </div>

        <!-- Test connection -->
        <div class="test-row">
          <button type="button" class="btn-test"
            onclick={testConnection} disabled={testing}>
            {testing ? 'Probando…' : 'Probar conexión'}
          </button>
          {#if testResult}
            <span class="test-result" class:ok={testResult.ok} class:fail={!testResult.ok}>
              {testResult.ok ? '✓' : '✕'} {testResult.message}
            </span>
          {/if}
        </div>
      </div>

      <div class="form-footer">
        <button type="submit" class="btn-save" disabled={saving}>
          {saving ? 'Guardando…' : 'Guardar configuración'}
        </button>
      </div>
    </div>

  </form>
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
    flex-direction: column;
    gap: 0.3rem;
  }

  .page-content {
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
    padding: 2.5rem 2rem;
    width: 100%;
    max-width: 1000px;
    margin: 0 auto;
  }

  .page-title  { font-size: 1.5rem; font-weight: 700; color: #fff; margin: 0; letter-spacing: -0.02em; }
  .page-sub    { font-size: 0.875rem; color: rgba(255, 255, 255, 0.8); margin: 0; max-width: 70ch; }

  .banner {
    border-radius: var(--radius-sm);
    padding: 0.75rem 1rem;
    font-size: 0.875rem;
  }
  .banner-error { background: var(--danger-lt); border-left: 3px solid var(--danger); color: var(--danger); }
  .banner-ok    { background: var(--success-lt); border-left: 3px solid var(--success); color: var(--success); }

  /* Two-column layout */
  .layout {
    display: grid;
    grid-template-columns: 280px 1fr;
    gap: 1.25rem;
    align-items: start;
  }

  .col-left, .col-right {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .card {
    background: var(--white);
    border: 1.5px solid var(--blue-100);
    border-radius: var(--radius-lg);
    padding: 1.5rem;
    box-shadow: var(--shadow-sm);
    display: flex;
    flex-direction: column;
    gap: 1.1rem;
  }

  .card-title {
    font-size: 0.9rem;
    font-weight: 700;
    color: var(--text);
    margin: 0;
    padding-bottom: 0.75rem;
    border-bottom: 1px solid var(--blue-100);
    letter-spacing: -0.01em;
  }

  /* Provider selector */
  .provider-list { display: flex; flex-direction: column; gap: 0.5rem; }

  .provider-row {
    display: flex;
    align-items: flex-start;
    gap: 0.75rem;
    border: 1.5px solid var(--blue-100);
    border-radius: var(--radius-md);
    padding: 0.875rem;
    cursor: pointer;
    transition: all 0.15s;
    text-transform: none;
    letter-spacing: 0;
    font-size: 1rem;
    font-weight: 400;
  }

  .provider-row:hover { border-color: var(--blue-300); }

  .provider-row.selected {
    border-color: var(--blue-500);
    background: var(--blue-50);
  }

  .provider-row input[type="radio"] {
    margin-top: 3px;
    flex-shrink: 0;
    accent-color: var(--blue-600);
    width: 16px;
    height: 16px;
  }

  .provider-info {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }

  .provider-info strong {
    font-size: 0.875rem;
    color: var(--text);
    font-weight: 600;
  }

  .provider-info span {
    font-size: 0.775rem;
    color: var(--muted);
    line-height: 1.45;
  }

  /* Fields */
  .field { display: flex; flex-direction: column; gap: 0.3rem; }

  label {
    font-size: 0.72rem;
    font-weight: 700;
    color: var(--muted);
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }

  input, select {
    background: var(--bg);
    border: 1.5px solid var(--blue-100);
    border-radius: var(--radius-sm);
    padding: 0.5rem 0.75rem;
    font-size: 0.875rem;
    color: var(--text);
    outline: none;
    transition: border-color 0.15s;
    width: 100%;
  }

  input:focus, select:focus { border-color: var(--blue-400); }

  .key-row { display: flex; gap: 0.5rem; }
  .key-row input { flex: 1; }

  .btn-toggle {
    background: none;
    border: 1.5px solid var(--blue-100);
    border-radius: var(--radius-sm);
    padding: 0.4rem 0.875rem;
    font-size: 0.8rem;
    color: var(--muted);
    cursor: pointer;
    white-space: nowrap;
    transition: all 0.15s;
    flex-shrink: 0;
  }
  .btn-toggle:hover { border-color: var(--blue-400); color: var(--blue-600); }

  .hint { font-size: 0.75rem; color: var(--muted); margin: 0; }

  /* Test connection */
  .test-row {
    display: flex;
    align-items: center;
    gap: 0.875rem;
    flex-wrap: wrap;
    padding-top: 0.25rem;
  }

  .btn-test {
    background: none;
    border: 1.5px solid var(--blue-200);
    border-radius: var(--radius-pill);
    padding: 0.45rem 1.1rem;
    font-size: 0.8rem;
    font-weight: 600;
    color: var(--blue-600);
    cursor: pointer;
    transition: all 0.15s;
    white-space: nowrap;
  }
  .btn-test:hover { background: var(--blue-50); border-color: var(--blue-400); }
  .btn-test:disabled { opacity: 0.5; cursor: not-allowed; }

  .test-result { font-size: 0.8rem; font-weight: 500; }
  .test-result.ok   { color: #6c824f; }
  .test-result.fail { color: #601313; }

  /* Footer */
  .form-footer { display: flex; justify-content: flex-end; }

  .btn-save {
    background: var(--blue-900);
    color: white;
    border: none;
    border-radius: var(--radius-pill);
    padding: 0.55rem 1.75rem;
    font-size: 0.875rem;
    font-weight: 700;
    cursor: pointer;
    transition: opacity 0.15s;
    box-shadow: 0 2px 8px rgba(37,99,235,.3);
  }
  .btn-save:disabled { opacity: 0.5; cursor: not-allowed; }
  .btn-save:not(:disabled):hover { opacity: 0.88; }

  @media (max-width: 700px) {
    .page { padding: 1.5rem; }
    .layout { grid-template-columns: 1fr; }
  }
</style>