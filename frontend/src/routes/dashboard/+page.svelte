<script lang="ts">
  import type { PageData } from './$types';
  import { getPermissions } from '$lib/permissions';

  let { data }: { data: PageData } = $props();

  const role  = $derived(data.user?.role ?? 'alumno');
  const perms = $derived(getPermissions(role));

  const roleInfo = $derived.by(() => {
    if (role === 'super_admin') return {
      eyebrow: 'Super Administrador',
      context_title: '¿Qué es AI Surveys?',
      context_body: `AI Surveys es la plataforma de retroalimentación conversacional de la organización.
        Como super administrador, tienes visibilidad total sobre todos los equipos,
        encuestas y resultados.`,
      context_highlight: 'Tú ves todo. Ningún equipo ni encuesta está fuera de tu alcance.',
    };
    if (role === 'admin') return {
      eyebrow: 'Administrador · Profesor o Staff',
      context_title: 'Deja de adivinar qué piensan tus estudiantes.',
      context_body: `AI Surveys recopila retroalimentación real sin formularios aburridos.
      La plataforma hace las preguntas por ti y te entrega análisis listos para usar.`,
      context_highlight: 'Al cerrar tu encuesta recibes resúmenes automáticos, análisis de sentimiento y temas principales.',
    };
    // Alumno nunca llega a /dashboard (la Fase 3 lo bloquea en el layout),
    // así que este fallback es efectivamente la vista de Profesor.
    return {
      eyebrow: 'Profesor',
      context_title: 'Sigue el pulso de tus grupos.',
      context_body: `Consulta las encuestas asignadas a tus grupos
      y da seguimiento a quién ya respondió.`,
      context_highlight: 'Puedes ver el estado de las encuestas de tus grupos, sin poder editarlas ni crear nuevas.',
    };
  });

  const canDoItems = $derived.by(() => {
    if (role === 'super_admin') return [
      { title: 'Monitorear todos los equipos', desc: 'Ve todos los equipos de la plataforma sin necesitar ser parte de ellos.' },
      { title: 'Supervisar todas las encuestas', desc: 'Accede a cualquier encuesta de cualquier departamento.' },
      { title: 'Archivar encuestas', desc: 'Solo tú puedes archivar. Limpia el panel sin perder datos históricos.' },
      { title: 'Ver resultados globales', desc: 'Exporta y analiza datos de cualquier encuesta activa o cerrada.' },
    ];
    if (role === 'admin') return [
      { title: 'Crear equipos e invitar colaboradores', desc: 'Organiza tu equipo, asígnales roles e invítalos por correo.' },
      { title: 'Diseñar encuestas', desc: 'Crea encuestas con 7 tipos de pregunta y modo conversacional.' },
      { title: 'Compartir por liga o código QR', desc: 'Activa tu encuesta y comparte el acceso instantáneamente.' },
      { title: 'Analizar resultados automáticamente', desc: 'Resúmenes por pregunta, análisis de sentimiento y temas recurrentes.' },
      { title: 'Duplicar encuestas', desc: 'Reutiliza encuestas anteriores como base para las recurrentes.' },
    ];
    return [
      { title: 'Ver encuestas de tus grupos', desc: 'Consulta las encuestas asignadas a los grupos donde participas.' },
      { title: 'Dar seguimiento a respuestas', desc: 'Revisa quién ya respondió y quién falta.' },
      { title: '¿Necesitas crear o editar encuestas?', desc: 'Pide a un Coordinador que lo haga por ti.', muted: true },
    ];
  });

  const quickItems = $derived.by(() => {
    if (role === 'super_admin') return [
      { href: '/dashboard/teams',    title: 'Equipos',            desc: 'Monitorea todos los equipos' },
      { href: '/dashboard/surveys',  title: 'Todas las encuestas', desc: 'Ve y archiva desde aquí' },
      { href: '/dashboard/settings', title: 'Configuración IA',    desc: 'Proveedor, API key y modelo', cta: true },
    ];
    if (role === 'admin') return [
      { href: '/dashboard/teams',       title: 'Crear equipo',   desc: 'Invita a tus colaboradores' },
      { href: '/dashboard/surveys/new', title: 'Nueva encuesta', desc: 'Configura preguntas y modo conversacional', cta: true },
      { href: '/dashboard/surveys',     title: 'Mis encuestas',  desc: 'Administra y revisa resultados' },
    ];
    return [
      { href: '/dashboard/surveys',   title: 'Ver encuestas', desc: 'Encuestas de tus grupos' },
      { href: '/dashboard/analytics', title: 'Analytics',     desc: 'Reportes y seguimiento (próximamente)' },
    ];
  });
</script>

<svelte:head><title>Inicio — AI Surveys</title></svelte:head>

<div class="page">

  <!-- ── Banda de bienvenida ──────────────────────────────────────── -->
  <section class="hero">
    <div class="hero-inner">
      <p class="hero-eyebrow">{roleInfo.eyebrow}</p>
      <h1 class="hero-heading">
        Hola, {data.user?.display_name?.split(' ')[0] ?? 'Usuario'}.
      </h1>
      <p class="hero-body">{roleInfo.context_body}</p>
      <blockquote class="hero-quote">{roleInfo.context_highlight}</blockquote>
    </div>
  </section>

  <!-- ── Lo que puedes hacer ──────────────────────────────────────── -->
  <section class="section">
    <div class="section-inner">
      <h2 class="section-label">Lo que puedes hacer</h2>
      <div class="card-grid">
        {#each canDoItems as item}
          <div class="feature-card" class:muted={item.muted}>
            <h3 class="feature-title">{item.title}</h3>
            <p class="feature-desc">{item.desc}</p>
          </div>
        {/each}
      </div>
    </div>
  </section>

  <!-- ── Accesos ──────────────────────────────────────────────────── -->
  <section class="section section-light">
    <div class="section-inner">
      <h2 class="section-label">
        {role === 'super_admin' ? 'Accesos de administración' : role === 'admin' ? 'Por dónde empezar' : 'Tus accesos'}
      </h2>
      <div class="quick-grid">
        {#each quickItems as item}
          <a href={item.href} class="quick-card" class:quick-card--cta={item.cta}>
            <span class="quick-arrow" aria-hidden="true"></span>
            <span class="quick-text">
              <strong>{item.title}</strong>
              <small>{item.desc}</small>
            </span>
          </a>
        {/each}
      </div>
    </div>
  </section>

</div>

<style>
  .page {
    display: flex;
    flex-direction: column;
    width: 100%;
  }

  /* ── Banda institucional ─────────────────────────────────────────── */
  .hero {
    background: var(--blue-700);
    padding: 3rem 2rem;
  }

  .hero-inner {
    max-width: var(--container);
    margin: 0 auto;
  }

  .hero-eyebrow {
    font-family: var(--font-display);
    font-size: 0.7rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.1em;
    color: var(--blue-400);
    margin: 0 0 0.75rem;
  }

  .hero-heading {
    font-size: 2.5rem;
    color: #fff;
    margin: 0 0 1rem;
    line-height: 1.1;
  }

  .hero-body {
    font-size: 0.9375rem;
    color: rgba(255, 255, 255, 0.85);
    line-height: 1.7;
    margin: 0 0 1.25rem;
    max-width: 62ch;
  }

  .hero-quote {
    font-size: 0.875rem;
    color: #fff;
    font-weight: 500;
    border-left: 3px solid var(--blue-400);
    padding-left: 0.875rem;
    margin: 0;
    line-height: 1.6;
    max-width: 62ch;
  }

  /* ── Secciones ───────────────────────────────────────────────────── */
  .section {
    padding: 2.75rem 2rem;
    background: var(--bg);
  }

  .section-light {
    background: var(--white);
    border-top: 1px solid var(--border);
  }

  .section-inner {
    max-width: var(--container);
    margin: 0 auto;
  }

  .section-label {
    font-family: var(--font-display);
    font-size: 0.72rem;
    font-weight: 700;
    color: var(--blue-900);
    text-transform: uppercase;
    letter-spacing: 0.1em;
    margin: 0 0 1.25rem;
    padding-bottom: 0.6rem;
    border-bottom: 2px solid var(--blue-400);
  }

  /* ── Tarjetas de capacidades ─────────────────────────────────────── */
  .card-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
    gap: 1rem;
  }

  .feature-card {
    background: var(--white);
    border: 1px solid var(--border);
    border-left: 3px solid var(--blue-400);
    border-radius: var(--radius-sm);
    padding: 1.25rem 1.375rem;
    box-shadow: var(--shadow-sm);
    transition: box-shadow 0.15s, border-color 0.15s;
  }

  .feature-card:hover {
    border-left-color: var(--blue-900);
    box-shadow: var(--shadow-md);
  }

  .feature-card.muted {
    opacity: 0.75;
    border-left-color: var(--border);
  }

  .feature-title {
    font-size: 0.9rem;
    font-weight: 600;
    color: var(--blue-900);
    margin: 0 0 0.3rem;
  }

  .feature-desc {
    font-size: 0.8125rem;
    color: var(--muted);
    line-height: 1.55;
    margin: 0;
  }

  /* ── Accesos rápidos ─────────────────────────────────────────────── */
  .quick-grid {
    display: flex;
    flex-wrap: wrap;
    gap: 1rem;
  }

  .quick-card {
    display: flex;
    align-items: flex-start;
    gap: 0.85rem;
    background: var(--blue-50);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    padding: 1.25rem 1.5rem;
    text-decoration: none;
    min-width: 230px;
    transition: background 0.15s, border-color 0.15s;
  }

  .quick-card:hover {
    border-color: var(--blue-400);
    background: var(--white);
  }

  /* Anillo con flecha: el motivo de enlace del sitio institucional. */
  .quick-arrow {
    width: 28px;
    height: 28px;
    flex-shrink: 0;
    margin-top: 0.1rem;
    background: var(--blue-400);
    -webkit-mask: var(--ring-arrow) center / contain no-repeat;
    mask: var(--ring-arrow) center / contain no-repeat;
  }

  .quick-text {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }

  .quick-card strong {
    font-family: var(--font-display);
    font-size: 0.9rem;
    font-weight: 600;
    color: var(--blue-900);
  }

  .quick-card small {
    font-size: 0.775rem;
    color: var(--muted);
    line-height: 1.4;
  }

  .quick-card--cta {
    background: var(--blue-900);
    border-color: var(--blue-900);
  }

  .quick-card--cta:hover {
    background: var(--blue-700);
    border-color: var(--blue-700);
  }

  .quick-card--cta strong { color: #fff; }
  .quick-card--cta small  { color: rgba(255, 255, 255, 0.8); }
  .quick-card--cta .quick-arrow { background: #fff; }

  @media (max-width: 700px) {
    .hero { padding: 2.5rem 1.25rem; }
    .hero-heading { font-size: 1.9rem; }
    .section { padding: 2rem 1.25rem; }
    .quick-card { min-width: 100%; }
  }
</style>
