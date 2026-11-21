<script lang="ts">
	import { onDestroy, tick } from 'svelte';
	import { fade, fly } from 'svelte/transition';
	import { askQuestion, citationHref, type Citation } from '../lib/api/chat.js';
	import ChatPanel from '#lib/components/ChatPanel.svelte';
	import JellyOrb from '#lib/components/JellyOrb.svelte';
	import WaveBackground from '#lib/components/WaveBackground.svelte';
	import MeltBackground from '#lib/components/MeltBackground.svelte';

	type Turn = { question: string; answer?: string; citations?: Citation[]; error?: string };
	const chatColors = ['#050807', '#07120e', '#08131a', '#0b0f08'];
	let listening = $state(false);
	let conversation = $state<Turn[]>([]);
	let referenceIndex = $state(0);
	let typing = $state(false);
	let conversationEl: HTMLDivElement | undefined = $state();
	let orbPlacement = $state({ left: 24, top: 24, size: 64 });
	let orbTracking = $state(false);
	let orbTrackTimer: ReturnType<typeof setTimeout> | undefined;
	const phase = $derived(conversation.length ? 'chat' : listening ? 'listening' : 'idle');
	const references = $derived(conversation[referenceIndex]?.citations ?? []);
	const backgroundIndex = $derived((conversation.length + 3) % chatColors.length);
	const orbClass = $derived(
		phase === 'chat'
			? ''
			: 'left-1/2 top-[28vh] h-64 w-64 -translate-x-1/2 -translate-y-1/2 sm:h-80 sm:w-80 ' +
					(listening ? 'scale-110' : 'scale-100')
	);

	function updateOrbPlacement(tracked = false) {
		if (phase !== 'chat' || !conversationEl) return;

		const turns = conversationEl.querySelectorAll<HTMLElement>('.chat-turn');
		const last = turns[turns.length - 1];
		if (!last) return;

		const target = last.querySelector<HTMLElement>('.chat-answer') ?? last;
		const rect = target.getBoundingClientRect();
		const log = conversationEl.querySelector<HTMLElement>('.chat-log');
		const edge = log ? log.getBoundingClientRect().left : rect.left;
		const available = edge - 24;

		if (available < 150) {
			orbPlacement = { left: 20, top: 20, size: 64 };
			return;
		}

		const size = Math.min(168, Math.max(96, available - 32));
		const left = Math.max(16, edge - size - 24);
		const top = Math.min(
			Math.max(16, rect.top + rect.height / 2 - size / 2),
			Math.max(16, window.innerHeight - size - 132)
		);
		orbPlacement = { left, top, size };

		if (tracked) {
			orbTracking = true;
			if (orbTrackTimer) clearTimeout(orbTrackTimer);
			orbTrackTimer = setTimeout(() => {
				orbTracking = false;
			}, 160);
		}
	}

	async function ask(question: string) {
		if (typing) return;
		const i = conversation.length;
		conversation = [...conversation, { question }];
		referenceIndex = i;
		typing = true;
		await tick();
		conversationEl?.scrollTo({ top: conversationEl.scrollHeight });
		updateOrbPlacement();
		try {
			conversation[i] = { question, ...(await askQuestion(question)) };
		} catch (failure) {
			conversation[i] = {
				question,
				error: failure instanceof Error ? failure.message : '问答失败，请重试。'
			};
		} finally {
			typing = false;
			await tick();
			conversationEl?.scrollTo({ top: conversationEl.scrollHeight });
			updateOrbPlacement();
		}
	}

	$effect(() => {
		const element = conversationEl;
		if (!element) return;
		const onChange = () => updateOrbPlacement(true);
		element.addEventListener('scroll', onChange, { passive: true });
		window.addEventListener('resize', onChange);
		return () => {
			element.removeEventListener('scroll', onChange);
			window.removeEventListener('resize', onChange);
		};
	});
	onDestroy(() => clearTimeout(orbTrackTimer));
</script>

<svelte:head>
	<title>RAGanswer</title>
</svelte:head>

<main class="relative min-h-dvh w-full overflow-x-hidden bg-[#050706] text-zinc-100">
	<a
		href="/admin"
		rel="external"
		class="fixed top-4 right-4 z-30 rounded-lg border border-white/20 bg-zinc-950/70 px-3 py-2 text-sm text-zinc-100 hover:border-emerald-300 focus-visible:outline-2"
	>
		管理后台
	</a>
	<div
		class={`pointer-events-none fixed inset-0 z-0 transition-all duration-1000 ease-out motion-reduce:transition-none ${
			phase === 'chat' ? 'opacity-100' : 'opacity-0'
		}`}
		style={`background-color: ${chatColors[backgroundIndex]};`}
		aria-hidden="true"
	></div>
	<div
		class={`pointer-events-none fixed inset-0 z-[1] transition-opacity duration-1000 ease-out motion-reduce:transition-none ${
			phase === 'idle' ? 'opacity-[0.14]' : 'opacity-[0.55]'
		}`}
		aria-hidden="true"
	>
		<MeltBackground revealed={true} />
	</div>
	<WaveBackground visible={phase !== 'chat'} />

	<div
		class={`pointer-events-none fixed z-20 transition-all duration-1000 ease-in-out ${orbClass}`}
		style={phase === 'chat'
			? `left:${orbPlacement.left}px; top:${orbPlacement.top}px; width:${orbPlacement.size}px; height:${orbPlacement.size}px;${
					orbTracking ? ' transition:none;' : ''
				}`
			: ''}
	>
		<JellyOrb />
	</div>

	{#if phase === 'chat'}
		<div class="chat-head" in:fade={{ duration: 400 }}>
			<span class="chat-brand">Orb</span>
			<span class="chat-head-meta">对话记录 · {conversation.length} 轮</span>
		</div>

		<div class="chat-shell" in:fade={{ duration: 450 }}>
			<section class="chat-main" aria-label="问答对话">
				<div bind:this={conversationEl} class="scrollbar-subtle chat-transcript" aria-live="polite">
					<ol class="chat-log">
						{#each conversation as turn, i (i)}
							<li class="chat-turn" in:fade={{ duration: 240 }}>
								<div class="chat-index">
									<span class="chat-num">{String(i + 1).padStart(2, '0')}</span>
									<span class="chat-role">提问</span>
								</div>

								<div class="chat-body">
									<p class="chat-ask">{turn.question}</p>

									{#if turn.answer}
										<div class="chat-reply" in:fade={{ duration: 300 }}>
											<p class="chat-signature">Orb</p>
											<p class="chat-answer whitespace-pre-wrap">{turn.answer}</p>
											{#if turn.citations?.length}
												<button
													type="button"
													class="mt-3 text-sm text-emerald-300 underline"
													aria-controls="references"
													aria-pressed={referenceIndex === i}
													onclick={() => (referenceIndex = i)}
												>
													查看出处（{turn.citations.length}）
												</button>
											{/if}
										</div>
									{:else if turn.error}
										<p role="alert" class="text-sm text-rose-200">{turn.error}</p>
									{/if}
								</div>
							</li>
						{/each}

						{#if typing}
							<li class="chat-turn" in:fade={{ duration: 240 }}>
								<div class="chat-index">
									<span class="chat-num">··</span>
									<span class="chat-role">检索</span>
								</div>
								<div class="chat-body">
									<div class="chat-reply">
										<p class="chat-signature is-waiting">
											Orb
											<span>正在检索知识库</span>
										</p>
										<div class="chat-loader" aria-hidden="true"><span></span></div>
									</div>
								</div>
							</li>
						{/if}
					</ol>
				</div>

				<div class="chat-arc">
					<div class="chat-arc-wash" aria-hidden="true"></div>
					<div class="relative z-10 w-full max-w-2xl px-5">
						<ChatPanel bind:listening pending={typing} onask={ask} />
					</div>
				</div>
			</section>

			<aside
				id="references"
				class="scrollbar-subtle chat-refs"
				in:fly={{ x: 80, duration: 700, delay: 120 }}
				aria-label="参考文献"
			>
				<div class="chat-refs-head">
					<h2>参考文献</h2>
					<span>{references.length ? String(references.length).padStart(2, '0') : '—'}</span>
				</div>
				<p class="chat-refs-note">知识库检索片段，按相关度排序。</p>

				{#if references.length}
					<ol class="chat-ref-list">
						{#each references as reference, i (reference.document_id + ':' + i)}
							{@const href = citationHref(reference.url)}
							<li>
								<span class="chat-ref-num">{String(i + 1).padStart(2, '0')}</span>
								<p class="chat-ref-source">{reference.page === null ? '网页' : 'PDF'}</p>
								<p class="chat-ref-title">{reference.title}</p>
								<p class="chat-ref-snippet">{reference.excerpt}</p>
								<div class="chat-ref-foot">
									<span>
										{[
											reference.version,
											reference.section,
											reference.page === null ? null : '第 ' + reference.page + ' 页'
										]
											.filter(Boolean)
											.join(' · ')}
									</span>
									{#if href}<a {href} rel="external" class="text-emerald-300 underline">
											打开原文
										</a>{/if}
								</div>
							</li>
						{/each}
					</ol>
				{:else}
					<p class="chat-refs-empty">{typing ? '正在检索…' : '这条回答没有出处。'}</p>
				{/if}
			</aside>
		</div>
	{:else}
		<section class="relative z-10 mx-auto w-full max-w-2xl px-6 pt-[48dvh] pb-8">
			<h1 class="text-center text-3xl font-medium text-zinc-50 sm:text-4xl">工厂知识问答</h1>
			<ChatPanel bind:listening onask={ask} />
		</section>
	{/if}
</main>

<style>
	.chat-head {
		position: fixed;
		left: 6rem;
		top: 2.1rem;
		z-index: 20;
		display: flex;
		align-items: baseline;
		gap: 0.85rem;
		pointer-events: none;
	}

	.chat-brand {
		font-family:
			'Iowan Old Style', 'Palatino Linotype', 'Book Antiqua', Georgia, 'Songti SC', 'STSong',
			'SimSun', serif;
		font-size: 1.05rem;
		letter-spacing: 0.02em;
		color: #ece7db;
	}

	.chat-head-meta {
		font-size: 0.6875rem;
		letter-spacing: 0.16em;
		color: #6c6a61;
	}

	.chat-shell {
		position: relative;
		z-index: 10;
		display: flex;
		min-height: 100dvh;
		flex-direction: column;
	}

	.chat-main {
		display: flex;
		min-height: 0;
		flex: 1;
		flex-direction: column;
	}

	.chat-transcript {
		min-height: 0;
		flex: 1;
		overflow-y: auto;
		padding: 7.5rem 1.5rem 0;
	}

	.chat-log {
		margin: 0 auto;
		width: 100%;
		max-width: 44rem;
		list-style: none;
		padding: 0;
	}

	.chat-turn {
		display: grid;
		grid-template-columns: 4.5rem minmax(0, 1fr);
		gap: 1.75rem;
		padding: 2.5rem 0;
		border-top: 1px solid rgba(236, 231, 219, 0.07);
	}

	.chat-turn:first-child {
		border-top: 0;
		padding-top: 0.5rem;
	}

	.chat-index {
		display: flex;
		flex-direction: column;
		gap: 0.35rem;
		padding-top: 0.3rem;
	}

	.chat-num {
		font-family: 'Iowan Old Style', Georgia, 'Songti SC', 'SimSun', serif;
		font-size: 0.8125rem;
		letter-spacing: 0.08em;
		color: #9caf97;
		font-variant-numeric: oldstyle-nums;
	}

	.chat-role {
		font-size: 0.625rem;
		letter-spacing: 0.2em;
		color: #5f5d55;
	}

	.chat-body {
		min-width: 0;
	}

	.chat-ask {
		margin: 0;
		font-family:
			'Iowan Old Style', 'Palatino Linotype', Georgia, 'Songti SC', 'STSong', 'SimSun', serif;
		font-size: 1.0625rem;
		line-height: 1.9;
		color: #efeae0;
	}

	.chat-reply {
		margin-top: 1.9rem;
	}

	.chat-signature {
		display: flex;
		align-items: center;
		gap: 0.7rem;
		margin: 0 0 0.9rem;
		font-size: 0.6875rem;
		letter-spacing: 0.14em;
		color: #9caf97;
	}

	.chat-signature::before {
		content: '';
		width: 1.1rem;
		height: 1px;
		background: currentColor;
		opacity: 0.55;
	}

	.chat-signature span {
		color: #6c6a61;
		letter-spacing: 0.06em;
	}

	.chat-signature.is-waiting span {
		color: #8a877c;
	}

	.chat-answer {
		margin: 0;
		font-family:
			'Iowan Old Style', 'Palatino Linotype', Georgia, 'Songti SC', 'STSong', 'SimSun', serif;
		font-size: 1rem;
		line-height: 1.95;
		color: #c9c4b8;
		text-wrap: pretty;
	}

	.chat-loader {
		position: relative;
		width: 10rem;
		height: 1px;
		margin-top: 0.4rem;
		background: rgba(236, 231, 219, 0.1);
		overflow: hidden;
	}

	.chat-loader span {
		position: absolute;
		inset: 0 auto 0 0;
		width: 40%;
		background: #9caf97;
		animation: chat-scan 1.4s cubic-bezier(0.6, 0, 0.4, 1) infinite;
	}

	@keyframes chat-scan {
		0% {
			transform: translateX(-100%);
		}
		100% {
			transform: translateX(350%);
		}
	}

	.chat-arc {
		position: relative;
		display: flex;
		align-items: flex-end;
		justify-content: center;
		flex: none;
		padding-bottom: 1.35rem;
	}

	.chat-arc-wash {
		position: absolute;
		bottom: 0;
		left: 50%;
		width: min(38rem, 94%);
		height: 7rem;
		transform: translateX(-50%);
		border-top: 1px solid rgba(236, 231, 219, 0.14);
		border-radius: 50% 50% 0 0 / 100% 100% 0 0;
		background: radial-gradient(
			ellipse at 50% 100%,
			rgba(156, 175, 151, 0.07),
			rgba(5, 7, 6, 0) 68%
		);
		pointer-events: none;
	}

	.chat-refs {
		min-height: 0;
		overflow-y: auto;
		padding: 1.5rem 1.25rem 2.5rem;
		border-top: 1px solid rgba(236, 231, 219, 0.09);
		background: rgba(7, 17, 12, 0.38);
		backdrop-filter: blur(28px) saturate(150%);
		-webkit-backdrop-filter: blur(28px) saturate(150%);
		box-shadow:
			inset 0 1px 0 rgba(255, 255, 255, 0.06),
			inset 1px 0 0 rgba(255, 255, 255, 0.04);
	}

	.chat-refs-head {
		display: flex;
		align-items: baseline;
		justify-content: space-between;
		gap: 1rem;
	}

	.chat-refs-head h2 {
		margin: 0;
		font-family:
			'Iowan Old Style', 'Palatino Linotype', Georgia, 'Songti SC', 'STSong', 'SimSun', serif;
		font-size: 0.95rem;
		font-weight: 500;
		letter-spacing: 0.08em;
		color: #e6e1d5;
	}

	.chat-refs-head > span {
		font-size: 0.625rem;
		letter-spacing: 0.16em;
		color: #6c6a61;
	}

	.chat-refs-note {
		margin: 0.55rem 0 0;
		font-size: 0.6875rem;
		line-height: 1.7;
		color: #66645c;
	}

	.chat-ref-list {
		margin: 1.6rem 0 0;
		padding: 0;
		list-style: none;
	}

	.chat-ref-list li {
		padding: 1.1rem 0;
		border-top: 1px solid rgba(236, 231, 219, 0.06);
	}

	.chat-ref-list li:first-child {
		border-top: 0;
		padding-top: 0;
	}

	.chat-ref-num {
		font-family: 'Iowan Old Style', Georgia, 'Songti SC', 'SimSun', serif;
		font-size: 0.6875rem;
		letter-spacing: 0.1em;
		color: #9caf97;
		font-variant-numeric: oldstyle-nums;
	}

	.chat-ref-source {
		margin: 0.35rem 0 0;
		font-size: 0.625rem;
		letter-spacing: 0.1em;
		color: #6f6d64;
	}

	.chat-ref-title {
		margin: 0.3rem 0 0;
		font-family:
			'Iowan Old Style', 'Palatino Linotype', Georgia, 'Songti SC', 'STSong', 'SimSun', serif;
		font-size: 0.9rem;
		line-height: 1.65;
		color: #d8d3c7;
	}

	.chat-ref-snippet {
		margin: 0.35rem 0 0;
		font-size: 0.75rem;
		line-height: 1.75;
		color: #8a867b;
	}

	.chat-ref-foot {
		display: flex;
		justify-content: space-between;
		gap: 1rem;
		margin-top: 0.55rem;
		font-size: 0.625rem;
		letter-spacing: 0.08em;
		color: #5a5850;
	}

	.chat-refs-empty {
		margin: 1.6rem 0 0;
		font-size: 0.75rem;
		color: #66645c;
	}

	@media (min-width: 640px) {
		.chat-transcript {
			padding-inline: 2.5rem;
		}

		.chat-refs {
			padding-inline: 1.75rem;
		}
	}

	@media (min-width: 1024px) {
		.chat-shell {
			height: 100dvh;
			flex-direction: row;
		}

		.chat-transcript {
			padding-left: 6rem;
		}

		.chat-refs {
			width: 20rem;
			height: 100%;
			flex-shrink: 0;
			overflow-y: auto;
			border-top: 0;
			border-left: 1px solid rgba(236, 231, 219, 0.09);
			padding-top: 1.75rem;
		}
	}

	@media (max-width: 640px) {
		.chat-transcript {
			padding-top: 6.5rem;
		}

		.chat-turn {
			grid-template-columns: minmax(0, 1fr);
			gap: 0.6rem;
			padding: 2rem 0;
		}

		.chat-index {
			flex-direction: row;
			align-items: baseline;
			gap: 0.6rem;
			padding-top: 0;
		}
	}
</style>
