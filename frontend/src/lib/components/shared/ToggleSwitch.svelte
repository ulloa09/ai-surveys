<script lang="ts">
  // Interruptor de acceso. Lo usan el maestro (toda la encuesta) y el de cada
  // equipo en el detalle de encuesta.
  //
  // Es un <button role="switch">, no un checkbox dentro de un <form>: la acción
  // se dispara al soltar el click y no hay nada que "guardar" después. Con
  // aria-checked, un lector de pantalla lo anuncia como interruptor y dice si
  // está prendido o apagado — un div con onclick no diría ninguna de las dos.

  interface Props {
    checked: boolean;
    /** Se llama con el estado DESEADO (el contrario del actual). */
    onchange: (next: boolean) => void;
    disabled?: boolean;
    busy?: boolean;
    /** Etiqueta accesible: qué prende/apaga este interruptor. */
    label: string;
    size?: 'md' | 'lg';
  }

  let { checked, onchange, disabled = false, busy = false, label, size = 'md' }: Props = $props();
</script>

<button
  type="button"
  role="switch"
  aria-checked={checked}
  aria-label={label}
  aria-busy={busy}
  class="switch switch--{size}"
  class:switch--on={checked}
  class:switch--busy={busy}
  disabled={disabled || busy}
  onclick={() => onchange(!checked)}
>
  <span class="knob"></span>
</button>

<style>
  .switch {
    --w: 42px;
    --h: 24px;
    --pad: 3px;
    position: relative;
    flex-shrink: 0;
    width: var(--w);
    height: var(--h);
    padding: 0;
    border: none;
    border-radius: 999px;
    background: var(--border, #d7dee8);
    cursor: pointer;
    transition: background 0.18s ease;
  }

  .switch--lg {
    --w: 54px;
    --h: 30px;
    --pad: 3.5px;
  }

  .switch--on {
    background: var(--blue-600, #2563eb);
  }

  .switch:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }

  .switch--busy {
    cursor: progress;
    opacity: 0.7;
  }

  /* El foco tiene que verse: este control se puede accionar con teclado y sin
     anillo no habría forma de saber dónde estás parado. */
  .switch:focus-visible {
    outline: 2px solid var(--blue-600, #2563eb);
    outline-offset: 2px;
  }

  .knob {
    position: absolute;
    top: var(--pad);
    left: var(--pad);
    width: calc(var(--h) - var(--pad) * 2);
    height: calc(var(--h) - var(--pad) * 2);
    border-radius: 50%;
    background: white;
    box-shadow: 0 1px 3px rgba(11, 44, 92, 0.3);
    transition: transform 0.18s ease;
  }

  .switch--on .knob {
    transform: translateX(calc(var(--w) - var(--h)));
  }

  @media (prefers-reduced-motion: reduce) {
    .switch,
    .knob {
      transition: none;
    }
  }
</style>
