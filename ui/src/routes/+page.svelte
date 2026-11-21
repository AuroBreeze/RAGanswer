<script lang="ts">
	import { onMount } from 'svelte';
	import { fade, fly } from 'svelte/transition';
	import JellyOrb from '#lib/components/JellyOrb.svelte';
	import WaveBackground from '#lib/components/WaveBackground.svelte';
	import MeltBackground from '#lib/components/MeltBackground.svelte';
	import { askQuestion, type Citation } from '#lib/api/chat.js';

	type Phase = 'idle' | 'listening' | 'chat';

	type Reference = {
		source: string;
		title: string;
		snippet: string;
		locator: string;
		score: string;
	};

	type Turn = {
		question: string;
		answer?: string;
		citations?: number[];
	};

	type MockTurn = Turn & {
		answer: string;
		citations: number[];
		references: Reference[];
	};

	const prompts = [
		'帮我总结一段文字',
		'解释一个技术概念',
		'写一个 Svelte 组件',
		'给我一些产品灵感'
	];

	const mockTurns: MockTurn[] = [
		{
			question: '帮我查一下三号线的设备维护记录。',
			answer:
				'三号线最近一次维护是 2026-11-12，工单 WO-20261112-03。完成了主轴轴承更换、联轴器对中与空载试运行，复检振动值 1.8 mm/s，低于 4.5 mm/s 的预警线。当前累计运行 1,842 小时，建议在 2,000 小时前安排下一次检查。',
			citations: [1, 2, 3],
			references: [
				{
					source: '设备维护台账 · 三号线',
					title: '2026-11-12 主轴轴承更换',
					snippet: '更换 SKF 6208-2RS 轴承，停机 3.5 小时，复检振动值 1.8 mm/s。',
					locator: '第 18 页 / 表 3-2',
					score: '0.94'
				},
				{
					source: '工单系统',
					title: 'WO-20261112-03',
					snippet: '任务：主轴异响排查；处理：更换轴承并完成对中；状态：已关闭。',
					locator: '工单详情',
					score: '0.89'
				},
				{
					source: '运行日志',
					title: '三号线累计运行时长',
					snippet: '截至 2026-11-20，累计运行 1,842 小时，日均 9.6 小时。',
					locator: '2026-11 月报',
					score: '0.82'
				}
			]
		},
		{
			question: '这个报警代码是什么意思？',
			answer:
				'E-217 表示冷却液流量不足。建议先检查冷却液液位、泵出口压力和管路过滤器；如果泵在运行但流量仍低，优先排查过滤器堵塞和管路泄漏。处理前不要继续高负荷运行。',
			citations: [1, 2],
			references: [
				{
					source: '设备报警手册 · 第 4 版',
					title: 'E-217 冷却液流量不足',
					snippet: '流量开关连续 5 秒低于阈值时触发；检查液位、泵与过滤器。',
					locator: '第 42 页',
					score: '0.96'
				},
				{
					source: '维护案例库',
					title: 'E-217 过滤器堵塞案例',
					snippet: '某产线因过滤器堵塞导致流量下降，清洗后恢复正常。',
					locator: '案例 #A-118',
					score: '0.84'
				}
			]
		},
		{
			question: '把今天的巡检结果整理成报告。',
			answer:
				'今日巡检共 12 项，10 项正常，2 项异常：3 号泵出口压力偏低（0.32 MPa，标准 ≥ 0.4 MPa），2 号机冷却液液位接近下限。建议先补充冷却液并安排泵检查，其余项目维持正常巡检节奏。',
			citations: [1, 2],
			references: [
				{
					source: '巡检记录 · 2026-11-21',
					title: '白班巡检结果',
					snippet: '12 项检查，2 项异常：3 号泵压力 0.32 MPa；2 号机液位低。',
					locator: '巡检表 #INS-1121',
					score: '0.93'
				},
				{
					source: '设备标准作业程序',
					title: '泵出口压力标准',
					snippet: '正常范围 0.4–0.6 MPa，低于 0.4 MPa 需检查。',
					locator: 'SOP-P-07',
					score: '0.88'
				}
			]
		},
		{
			question: '上次更换轴承是什么时候？',
			answer:
				'上次更换轴承是 2026-11-12，型号 SKF 6208-2RS，工单 WO-20261112-03。更换后空载试运行 30 分钟，复检振动值 1.8 mm/s，运行状态正常。',
			citations: [1, 2],
			references: [
				{
					source: '备件更换记录',
					title: 'SKF 6208-2RS 更换',
					snippet: '2026-11-12 更换主轴轴承，批次 B-2026-11。',
					locator: '备件台账 / 第 6 页',
					score: '0.95'
				},
				{
					source: '工单系统',
					title: 'WO-20261112-03',
					snippet: '更换后空载试运行 30 分钟，振动值 1.8 mm/s。',
					locator: '工单详情',
					score: '0.90'
				}
			]
		}
	];

	function toReference(citation: Citation): Reference {
		const locator = [
			citation.version,
			citation.section,
			citation.page == null ? null : `第 ${citation.page} 页`
		]
			.filter(Boolean)
			.join(' · ');
		return {
			source: citation.title,
			title: citation.section || citation.title,
			snippet: citation.excerpt,
			locator: locator || '知识库',
			score: '—'
		};
	}

	const chatColors = ['#050807', '#07120e', '#08131a', '#0b0f08'];

	let promptIndex = $state(0);
	let phase = $state<Phase>('idle');
	let speech = $state('');
	let turnIndex = $state(0);
	let conversation = $state<Turn[]>([]);
	let references = $state<Reference[]>([]);
	let typing = $state(false);
	let backgroundIndex = $state(0);
	let conversationEl: HTMLDivElement | undefined = $state();
	let inputTimer: ReturnType<typeof setTimeout> | undefined;
	let orbPlacement = $state({ left: 24, top: 24, size: 64 });
	let orbTracking = $state(false);
	let orbTrackTimer: ReturnType<typeof setTimeout> | undefined;

	const orbClass = $derived(
		phase === 'chat'
			? ''
			: `left-1/2 h-64 w-64 -translate-x-1/2 sm:h-80 sm:w-80 ${
					phase === 'listening'
						? 'top-[28vh] -translate-y-1/2 scale-110'
						: 'top-[30vh] -translate-y-1/2 scale-100'
				}`
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

	function clearInputTimer() {
		if (inputTimer) {
			clearTimeout(inputTimer);
			inputTimer = undefined;
		}
	}

	function handleScreenTap() {
		if (phase !== 'idle') return;
		speech = mockTurns[0].question;
		turnIndex = 0;
		phase = 'listening';
		clearInputTimer();
		inputTimer = setTimeout(enterChat, 2000);
	}

	async function enterChat() {
		const query = speech.trim() || mockTurns[0].question;
		phase = 'chat';
		conversation = [{ question: query }];
		references = [];
		typing = true;
		clearInputTimer();
		try {
			const result = await askQuestion(query);
			conversation = [
				{
					question: query,
					answer: result.answer,
					citations: result.citations.map((_, i) => i + 1)
				}
			];
			references = result.citations.map(toReference);
		} catch (failure) {
			conversation = [
				{
					question: query,
					answer: failure instanceof Error ? failure.message : '问答失败，请重试。',
					citations: []
				}
			];
		} finally {
			typing = false;
		}
	}

	async function handleChatInput() {
		if (phase !== 'chat' || typing) return;
		turnIndex = (turnIndex + 1) % mockTurns.length;
		const query = mockTurns[turnIndex].question;
		conversation = [...conversation, { question: query }];
		typing = true;
		backgroundIndex = (backgroundIndex + 1) % chatColors.length;
		try {
			const result = await askQuestion(query);
			conversation = conversation.map((turn, i) =>
				i === conversation.length - 1
					? {
							...turn,
							answer: result.answer,
							citations: result.citations.map((_, idx) => idx + 1)
						}
					: turn
			);
			references = result.citations.map(toReference);
		} catch (failure) {
			conversation = conversation.map((turn, i) =>
				i === conversation.length - 1
					? {
							...turn,
							answer: failure instanceof Error ? failure.message : '问答失败，请重试。',
							citations: []
						}
					: turn
			);
		} finally {
			typing = false;
		}
	}

	onMount(() => {
		const timer = setInterval(() => {
			promptIndex = (promptIndex + 1) % prompts.length;
		}, 3200);

		return () => {
			clearInterval(timer);
			clearInputTimer();
			if (orbTrackTimer) clearTimeout(orbTrackTimer);
		};
	});

	$effect(() => {
		const el = conversationEl;
		if (!el || conversation.length === 0) return;
		const behavior = typing ? 'smooth' : 'auto';
		el.scrollTo({ top: el.scrollHeight, behavior });
	});

	$effect(() => {
		if (phase !== 'chat' || !conversationEl) return;
		updateOrbPlacement();
		const frame = requestAnimationFrame(() => updateOrbPlacement());
		const onScroll = () => updateOrbPlacement(true);
		const onResize = () => updateOrbPlacement(true);
		conversationEl.addEventListener('scroll', onScroll, { passive: true });
		window.addEventListener('resize', onResize);
		return () => {
			cancelAnimationFrame(frame);
			conversationEl?.removeEventListener('scroll', onScroll);
			window.removeEventListener('resize', onResize);
		};
	});

	$effect(() => {
		const turns = conversation.length;
		const refs = references.length;
		const busy = typing;
		if (phase !== 'chat' || (turns === 0 && refs === 0 && !busy)) return;
		requestAnimationFrame(() => updateOrbPlacement());
	});
</script>

<svelte:head>
	<title>Orb Chat</title>
</svelte:head>

<main class="relative min-h-dvh w-full overflow-x-hidden bg-[#050706] text-zinc-100">
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
	<WaveBackground ontap={handleScreenTap} visible={phase !== 'chat'} />

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
			<section class="chat-main">
				<div bind:this={conversationEl} class="scrollbar-subtle chat-transcript">
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
											<p class="chat-signature">
												Orb
												<span>知识库 · {turn.citations?.length ?? 0} 条</span>
											</p>
											<p class="chat-answer">
												{turn.answer}
											</p>
										</div>
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
					<button
						type="button"
						onclick={handleChatInput}
						disabled={typing}
						class="chat-arc-button"
						aria-label="继续提问"
					>
						<svg
							xmlns="http://www.w3.org/2000/svg"
							fill="none"
							viewBox="0 0 24 24"
							stroke-width="1.6"
							stroke="currentColor"
							aria-hidden="true"
						>
							<path stroke-linecap="round" stroke-linejoin="round" d="M12 19V5m0 0-6 6m6-6 6 6" />
						</svg>
					</button>
				</div>
			</section>

			<aside
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
						{#each references as reference, i}
							<li>
								<span class="chat-ref-num">{String(i + 1).padStart(2, '0')}</span>
								<p class="chat-ref-source">{reference.source}</p>
								<p class="chat-ref-title">{reference.title}</p>
								<p class="chat-ref-snippet">{reference.snippet}</p>
								<div class="chat-ref-foot">
									<span>{reference.locator}</span>
									<span>{reference.score}</span>
								</div>
							</li>
						{/each}
					</ol>
				{:else}
					<p class="chat-refs-empty">正在检索…</p>
				{/if}
			</aside>
		</div>
	{:else}
		<header class="relative z-10 flex items-center justify-between px-5 py-4 sm:px-8">
			<div class="flex items-center gap-3">
				<span class="text-sm font-medium text-zinc-200">New chat</span>
				<span class="rounded-full border border-white/10 px-2.5 py-1 text-[11px] text-zinc-400">
					Orb 1.0
				</span>
			</div>
			<span class="text-[11px] text-zinc-600">RAG 预览</span>
		</header>

		<div
			class="pointer-events-none fixed left-1/2 top-[52vh] z-10 w-full max-w-2xl -translate-x-1/2 px-6 text-center"
		>
			{#if phase === 'listening'}
				<div in:fade={{ duration: 300 }}>
					<p class="text-[11px] uppercase tracking-[0.28em] text-emerald-400/70">你说</p>
					<p class="mt-3 text-2xl font-medium leading-snug tracking-tight text-zinc-50 sm:text-3xl">
						{speech}
					</p>
					<p class="mt-6 flex items-center justify-center gap-2 text-sm text-zinc-400">
						<span class="h-1.5 w-1.5 animate-pulse rounded-full bg-emerald-400" aria-hidden="true"></span>
						正在识别…
					</p>
				</div>
			{:else}
				<h1 class="text-3xl font-medium tracking-tight text-zinc-50 sm:text-4xl">你可以问…</h1>

				<div
					class="mt-4 flex min-h-7 items-center justify-center text-base text-zinc-300 sm:text-lg"
				>
					{#key promptIndex}
						<span in:fade={{ duration: 240 }} class="text-center">{prompts[promptIndex]}</span>
					{/key}
				</div>

				<p class="mt-6 flex select-none items-center justify-center gap-2 text-sm text-zinc-400">
					<span class="h-1.5 w-1.5 animate-pulse rounded-full bg-emerald-400" aria-hidden="true"></span>
					触摸屏幕来输入语音
				</p>
			{/if}
		</div>

		<footer
			class="absolute bottom-5 left-0 right-0 z-10 px-6 text-center text-[11px] text-zinc-600"
		>
			Orb 可能会出错，请核对重要信息。
		</footer>
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
		font-family: 'Iowan Old Style', 'Palatino Linotype', 'Book Antiqua', Georgia, 'Songti SC',
			'STSong', 'SimSun', serif;
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
		font-family: 'Iowan Old Style', 'Palatino Linotype', Georgia, 'Songti SC', 'STSong', 'SimSun',
			serif;
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
		font-family: 'Iowan Old Style', 'Palatino Linotype', Georgia, 'Songti SC', 'STSong', 'SimSun',
			serif;
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
		height: 7.5rem;
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

	.chat-arc-button {
		position: relative;
		z-index: 1;
		display: grid;
		place-items: center;
		width: 4.5rem;
		height: 4.5rem;
		margin-bottom: 0.2rem;
		border: 1px solid rgba(236, 231, 219, 0.16);
		border-radius: 9999px;
		background: rgba(8, 12, 10, 0.72);
		color: #cfcabe;
		cursor: pointer;
		transition:
			color 0.4s ease,
			border-color 0.4s ease,
			background 0.4s ease,
			transform 0.4s ease;
	}

	.chat-arc-button svg {
		width: 1.15rem;
		height: 1.15rem;
		transition: transform 0.4s ease;
	}

	.chat-arc-button:not(:disabled):hover {
		color: #ece7db;
		border-color: rgba(156, 175, 151, 0.55);
		background: rgba(12, 18, 14, 0.85);
		transform: translateY(-2px);
	}

	.chat-arc-button:not(:disabled):hover svg {
		transform: translateY(-2px);
	}

	.chat-arc-button:disabled {
		opacity: 0.35;
		cursor: default;
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
		font-family: 'Iowan Old Style', 'Palatino Linotype', Georgia, 'Songti SC', 'STSong', 'SimSun',
			serif;
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
		font-family: 'Iowan Old Style', 'Palatino Linotype', Georgia, 'Songti SC', 'STSong', 'SimSun',
			serif;
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
