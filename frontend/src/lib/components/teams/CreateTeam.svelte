<script lang="ts">
  let {
    onCreateTeam = (_name: string) => {},
    loading = false,
    error = ''
  }: {
    onCreateTeam?: (name: string) => void;
    loading?: boolean;
    error?: string;
  } = $props();

  let name = $state('');

  function handleSubmit(e: Event) {
    e.preventDefault();
    if (!name.trim()) return;
    onCreateTeam(name.trim());
    name = '';
  }
</script>

<form class="form" onsubmit={handleSubmit}>
  <div class="fields">
    <div class="field">
      <label for="team-name">Nombre del equipo</label>
      <input
        id="team-name"
        type="text"
        placeholder="Ej. Departamento de Ingeniería"
        bind:value={name}
        required
        disabled={loading}
      />
    </div>
    <button type="submit" class="btn-create" disabled={loading || !name.trim()}>
      {loading ? 'Creando…' : '+ Crear equipo'}
    </button>
  </div>
  {#if error}
    <p class="error">{error}</p>
  {/if}
</form>

<style>
  .form { width: 100%; }

  .fields {
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
    background: var(--white);
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

  .btn-create {
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
    height: fit-content;
  }

  .btn-create:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .btn-create:not(:disabled):hover { opacity: 0.85; }

  .error {
    margin-top: 0.5rem;
    font-size: 0.8rem;
    color: #dc2626;
  }
</style>
