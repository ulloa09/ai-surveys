<!-- src/lib/components/conversation/ChatBubble.svelte -->
<!-- Burbuja de mensaje para la UI conversacional del respondiente -->
<script lang="ts">
	type Role = 'user' | 'assistant' | 'system';

	let {
		role,
		content = '',
		streaming = false,
		onEdit = null
	}: {
		role: Role;
		content?: string;
		streaming?: boolean;
		onEdit?: (() => void) | null;
	} = $props();

	function formatContent(text: string): string {
		return text
			.replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
			.replace(/\*(.*?)\*/g, '<em>$1</em>')
			.replace(/\n/g, '<br>');
	}
</script>

{#if role === 'system'}
	<div class="msg-system">{@html formatContent(content)}</div>
{:else if role === 'assistant'}
	<div class="msg-row msg-ai">
		<span class="avatar" aria-hidden="true">AI</span>
		<div class="bubble ai-bubble">
			{#if streaming && !content}
				<span class="typing" aria-label="La IA está escribiendo">
					<span></span><span></span><span></span>
				</span>
			{:else}
				{@html formatContent(content)}
				{#if streaming}
					<span class="cursor" aria-hidden="true">▋</span>
				{/if}
			{/if}
		</div>
	</div>
{:else}
	<div class="msg-row msg-user">
		<div class="bubble user-bubble">{content}</div>
		{#if onEdit}
			<button
				class="btn-edit"
				onclick={onEdit}
				title="Editar respuesta"
				aria-label="Editar respuesta"
			>
				<svg
					width="12"
					height="12"
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					stroke-width="2.5"
					stroke-linecap="round"
					stroke-linejoin="round"
				>
					<path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" />
					<path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" />
				</svg>
			</button>
		{/if}
	</div>
{/if}

<style>
	.msg-system {
		text-align: center;
		font-size: 0.8rem;
		color: var(--muted);
		padding: 0.4rem 0.75rem;
		background: rgba(255, 255, 255, 0.7);
		border-radius: 20px;
		border: 1px solid var(--border);
		align-self: center;
		max-width: 80%;
	}

	.msg-row {
		display: flex;
		gap: 0.6rem;
		align-items: flex-end;
	}

	.msg-ai {
		align-self: flex-start;
		max-width: 85%;
	}
	.msg-user {
		align-self: flex-end;
		flex-direction: row-reverse;
		max-width: 75%;
	}

	.avatar {
		width: 28px;
		height: 28px;
		border-radius: 50%;
		background: var(--blue-900);
		color: white;
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
		font-family: var(--font-display);
		font-size: 0.65rem;
		font-weight: 700;
		letter-spacing: 0.02em;
	}

	.bubble {
		padding: 0.7rem 0.9rem;
		border-radius: var(--radius-md);
		font-size: 0.925rem;
		line-height: 1.6;
	}

	.ai-bubble {
		background: white;
		border: 1px solid var(--border);
		border-bottom-left-radius: 4px;
		box-shadow: 0 1px 3px rgba(11, 44, 92, 0.06);
	}

	.user-bubble {
		background: var(--blue-900);
		color: white;
		border-bottom-right-radius: 4px;
	}

	/* Botón editar — aparece junto a la burbuja del usuario */
	.btn-edit {
		background: var(--white, white);
		border: 1.5px solid var(--blue-700);
		border-radius: 6px;
		padding: 0.35rem;
		color: var(--blue-700);
		cursor: pointer;
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
		transition:
			background 0.15s,
			border-color 0.15s,
			color 0.15s;
		align-self: center;
	}

	.btn-edit:hover {
		background: var(--blue-100);
		border-color: var(--blue-900);
		color: var(--blue-900);
	}

	.typing {
		display: inline-flex;
		gap: 3px;
		align-items: center;
		height: 1.2em;
	}

	.typing span {
		width: 5px;
		height: 5px;
		border-radius: 50%;
		background: var(--blue-400);
		animation: bounce 1.2s infinite;
	}

	.typing span:nth-child(2) {
		animation-delay: 0.2s;
	}
	.typing span:nth-child(3) {
		animation-delay: 0.4s;
	}

	@keyframes bounce {
		0%,
		60%,
		100% {
			transform: translateY(0);
		}
		30% {
			transform: translateY(-5px);
		}
	}

	.cursor {
		color: var(--blue-400);
		animation: blink 1s step-end infinite;
	}

	@keyframes blink {
		0%,
		100% {
			opacity: 1;
		}
		50% {
			opacity: 0;
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.typing span,
		.cursor {
			animation: none;
		}
	}
</style>
