<script lang="ts">
  // Pie de página institucional: contacto, mapa del sitio y aviso legal.
</script>

<footer class="footer">
  <div class="footer-inner">

    <div class="col col--contact">
      <h2>CONTACTO</h2>

      <h3>AI Surveys</h3>
      <p class="org">Plataforma de encuestas conversacionales asistidas por IA.</p>
      <p class="phone"><a href="mailto:support@example.com">support@example.com</a></p>
    </div>

    <nav class="col">
      <h2>LA PLATAFORMA</h2>
      <ul>
        <li><a href="/dashboard">Panel principal</a></li>
        <li><a href="/dashboard/surveys">Encuestas</a></li>
        <li><a href="/dashboard/teams">Equipos</a></li>
        <li><a href="/dashboard/analytics">Resultados</a></li>
      </ul>
    </nav>

    <nav class="col">
      <h2>SOBRE EL PROYECTO</h2>
      <ul>
        <li><a href="https://github.com/ulloa09/ai-surveys/blob/main/docs/PRD.md" rel="noopener" target="_blank">Documento de producto</a></li>
        <li><a href="https://github.com/ulloa09/ai-surveys/blob/main/docs/milestones.md" rel="noopener" target="_blank">Plan de fases</a></li>
        <li><a href="https://github.com/ulloa09/ai-surveys/blob/main/docs/diagrams.md" rel="noopener" target="_blank">Arquitectura</a></li>
      </ul>
    </nav>

    <nav class="col">
      <h2>SERVICIOS</h2>
      <ul>
        <li><a href="mailto:support@example.com">Soporte</a></li>
        <li><a href="https://github.com/ulloa09/ai-surveys/tree/main/docs/issues" rel="noopener" target="_blank">Roadmap</a></li>
        <li><a href="https://github.com/ulloa09/ai-surveys/blob/main/LICENSE" rel="noopener" target="_blank">Licencia</a></li>
      </ul>
    </nav>

  </div>

  <div class="footer-legal">
    <div class="legal-inner">
      <p>© {new Date().getFullYear()} AI Surveys Platform.</p>
    </div>
  </div>
</footer>

<style>
  .footer {
    background: var(--blue-700);
    color: #fff;
    font-size: 0.875rem;
  }

  .footer-inner {
    max-width: var(--container);
    margin: 0 auto;
    padding: 3rem 2rem 2.5rem;
    display: grid;
    grid-template-columns: 1.4fr 1fr 1fr 1fr;
    gap: 2.5rem;
  }

  h2 {
    font-family: var(--font-display);
    font-size: 0.8rem;
    font-weight: 700;
    letter-spacing: 0.08em;
    color: #fff;
    margin: 0 0 1.25rem;
  }

  h3 {
    font-family: var(--font-display);
    font-size: 1rem;
    font-weight: 600;
    color: #fff;
    margin: 1.75rem 0 0.5rem;
  }

  .col ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  .col li { margin-bottom: 0.6rem; }

  .col a {
    color: rgba(255, 255, 255, 0.85);
    text-decoration: none;
  }

  .col a:hover {
    color: #fff;
    text-decoration: underline;
  }

  .org {
    color: var(--blue-400);
    font-weight: 600;
    font-size: 1rem;
    line-height: 1.35;
    margin: 0 0 0.9rem;
    max-width: 22rem;
  }

  .phone {
    margin: 0.9rem 0 0;
  }

  .phone a {
    color: rgba(255, 255, 255, 0.85);
    text-decoration: none;
    font-weight: 600;
  }

  .footer-legal {
    border-top: 1px solid rgba(255, 255, 255, 0.18);
  }

  .legal-inner {
    max-width: var(--container);
    margin: 0 auto;
    padding: 1.25rem 2rem;
  }

  .legal-inner p {
    margin: 0;
    font-size: 0.78rem;
    color: rgba(255, 255, 255, 0.7);
  }

  @media (max-width: 900px) {
    .footer-inner {
      grid-template-columns: 1fr 1fr;
      gap: 2rem;
      padding: 2.5rem 1.25rem 2rem;
    }
  }

  @media (max-width: 560px) {
    .footer-inner { grid-template-columns: 1fr; }
    .legal-inner { padding: 1.25rem; }
  }
</style>
