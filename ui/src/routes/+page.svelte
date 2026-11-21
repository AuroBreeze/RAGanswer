<script lang="ts">
	import { onMount } from 'svelte';
	import { fade, fly } from 'svelte/transition';
	import JellyOrb from '#lib/components/JellyOrb.svelte';
	import WaveBackground from '#lib/components/WaveBackground.svelte';
	import MeltBackground from '#lib/components/MeltBackground.svelte';

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

	const orbClass = $derived(
		phase === 'chat'
			? 'left-6 top-6 h-16 w-16 translate-x-0 translate-y-0 scale-100'
			: `left-1/2 h-64 w-64 -translate-x-1/2 sm:h-80 sm:w-80 ${
					phase === 'listening'
						? 'top-[28vh] -translate-y-1/2 scale-110'
						: 'top-[30vh] -translate-y-1/2 scale-100'
				}`
	);

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

	function enterChat() {
		phase = 'chat';
		conversation = [{ question: speech }];
		references = [];
		typing = true;
		clearInputTimer();
		inputTimer = setTimeout(() => {
			conversation = [
				{
					question: speech,
					answer: mockTurns[0].answer,
					citations: mockTurns[0].citations
				}
			];
			references = mockTurns[0].references;
			typing = false;
		}, 900);
	}

	function handleChatInput() {
		if (phase !== 'chat' || typing) return;
		turnIndex = (turnIndex + 1) % mockTurns.length;
		const next = mockTurns[turnIndex];
		conversation = [...conversation, { question: next.question }];
		typing = true;
		backgroundIndex = (backgroundIndex + 1) % chatColors.length;
		clearInputTimer();
		inputTimer = setTimeout(() => {
			conversation = conversation.map((turn, i) =>
				i === conversation.length - 1
					? { ...turn, answer: next.answer, citations: next.citations }
					: turn
			);
			references = next.references;
			typing = false;
		}, 900);
	}

	onMount(() => {
		const timer = setInterval(() => {
			promptIndex = (promptIndex + 1) % prompts.length;
		}, 3200);

		return () => {
			clearInterval(timer);
			clearInputTimer();
		};
	});

	$effect(() => {
		const el = conversationEl;
		if (!el || conversation.length === 0) return;
		const behavior = typing ? 'smooth' : 'auto';
		el.scrollTo({ top: el.scrollHeight, behavior });
	});
</script>

<svelte:head>
	<title>RAGanswer</title>
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
		class="pointer-events-none fixed inset-0 z-0 opacity-[0.14]"
		aria-hidden="true"
	>
		<MeltBackground revealed={true} />
	</div>
	<WaveBackground ontap={handleScreenTap} visible={phase !== 'chat'} />
	<div
		class={`pointer-events-none fixed inset-0 z-[1] transition-opacity duration-[1600ms] ease-out motion-reduce:transition-none ${
			phase === 'listening' ? 'opacity-100' : 'opacity-0'
		}`}
		style="background: radial-gradient(ellipse at 50% 45%, rgba(3, 6, 5, 0.55), rgba(3, 6, 5, 0.12) 62%, transparent 82%);"
		aria-hidden="true"
	></div>

	<div
		class={`pointer-events-none fixed z-20 transition-all duration-1000 ease-in-out motion-reduce:transition-none ${orbClass}`}
	>
		<JellyOrb />
	</div>

	{#if phase === 'chat'}
		<div
			class="pointer-events-none fixed left-24 top-9 z-20 flex items-baseline gap-2"
			in:fade={{ duration: 400 }}
		>
			<span class="text-sm font-medium text-zinc-200">Orb 1.0</span>
			<span class="text-[11px] text-zinc-500">RAG 对话</span>
		</div>

		<div
			class="relative z-10 flex min-h-dvh flex-col lg:h-dvh lg:flex-row"
			in:fade={{ duration: 450 }}
		>
			<section class="flex min-h-0 flex-1 flex-col">
				<div
					bind:this={conversationEl}
					class="min-h-0 flex-1 overflow-y-auto px-5 pt-24 sm:px-10 lg:pl-24"
				>
					<div class="mx-auto w-full max-w-2xl space-y-10 pb-6">
						{#each conversation as turn, i (i)}
							<div class="flex flex-col items-end" in:fade={{ duration: 240 }}>
								<span class="text-[11px] text-zinc-500">你</span>
								<p class="mt-2 max-w-xl text-right text-lg leading-relaxed text-zinc-100">
									{turn.question}
								</p>
							</div>
							{#if turn.answer}
								<div class="border-l border-emerald-400/30 pl-5" in:fade={{ duration: 300 }}>
									<div
										class="flex items-center gap-2 text-[11px] uppercase tracking-[0.22em] text-emerald-400/80"
									>
										<span class="h-1.5 w-1.5 rounded-full bg-emerald-400"></span>
										Orb
									</div>
									<p class="mt-3 text-base leading-7 text-zinc-300">{turn.answer}</p>
									{#if turn.citations?.length}
										<div class="mt-3 flex flex-wrap gap-1.5">
											{#each turn.citations ?? [] as citation}
												<span class="text-[11px] text-emerald-400/70">[{citation}]</span>
											{/each}
										</div>
									{/if}
								</div>
							{/if}
						{/each}
						{#if typing}
							<div class="border-l border-emerald-400/30 pl-5" in:fade={{ duration: 240 }}>
								<div
									class="flex items-center gap-2 text-[11px] uppercase tracking-[0.22em] text-emerald-400/80"
								>
									<span class="h-1.5 w-1.5 animate-pulse rounded-full bg-emerald-400"></span>
									Orb 正在检索
								</div>
								<div class="mt-4 flex gap-1.5">
									<span
										class="h-1.5 w-1.5 animate-bounce rounded-full bg-zinc-500 [animation-delay:-0.3s]"
									></span>
									<span
										class="h-1.5 w-1.5 animate-bounce rounded-full bg-zinc-500 [animation-delay:-0.15s]"
									></span>
									<span class="h-1.5 w-1.5 animate-bounce rounded-full bg-zinc-500"></span>
								</div>
							</div>
						{/if}
					</div>
				</div>

				<div class="shrink-0 px-5 pb-6 pt-3 sm:px-10 lg:pl-24">
					<button
						type="button"
						onclick={handleChatInput}
						disabled={typing}
						class="flex items-center gap-2 rounded-full border border-emerald-400/20 bg-emerald-400/10 px-5 py-2.5 text-sm text-emerald-300 transition hover:bg-emerald-400/15 disabled:cursor-not-allowed disabled:opacity-40"
					>
						<svg
							xmlns="http://www.w3.org/2000/svg"
							fill="none"
							viewBox="0 0 24 24"
							stroke-width="1.7"
							stroke="currentColor"
							class="h-4 w-4"
							aria-hidden="true"
						>
							<path
								stroke-linecap="round"
								stroke-linejoin="round"
								d="M12 18.75a6 6 0 0 0 6-6v-1.5m-6 7.5a6 6 0 0 1-6-6v-1.5m6 7.5v3.75m-3.75 0h7.5M12 15.75a3 3 0 0 1-3-3V4.5a3 3 0 1 1 6 0v8.25a3 3 0 0 1-3 3Z"
							/>
						</svg>
						{typing ? '正在检索…' : '轻触说话'}
					</button>
				</div>
			</section>

			<aside
				class="min-h-0 w-full overflow-y-auto border-t border-white/10 bg-[#070b09]/80 px-5 pb-10 pt-6 backdrop-blur-xl lg:h-full lg:w-80 lg:border-l lg:border-t-0"
				in:fly={{ x: 80, duration: 700, delay: 120 }}
				aria-label="参考文献"
			>
				<div class="flex items-baseline justify-between">
					<h2 class="text-sm font-medium text-zinc-200">参考文献</h2>
					<span class="text-[11px] text-zinc-500">
						{references.length ? references.length + ' 条' : '检索中'}
					</span>
				</div>
				<p class="mt-2 text-[11px] leading-5 text-zinc-500">来自知识库的检索片段，按相关度排序。</p>
				{#if references.length}
					<ol class="mt-6 space-y-6">
						{#each references as reference, i}
							<li class="border-l border-white/10 pl-4">
								<p class="text-[11px] text-emerald-400/70">[{i + 1}] {reference.source}</p>
								<p class="mt-1.5 text-sm leading-6 text-zinc-200">{reference.title}</p>
								<p class="mt-1 text-xs leading-5 text-zinc-500">{reference.snippet}</p>
								<div class="mt-2 flex justify-between text-[11px] text-zinc-600">
									<span>{reference.locator}</span>
									<span>{reference.score}</span>
								</div>
							</li>
						{/each}
					</ol>
				{:else}
					<p class="mt-6 text-xs text-zinc-500">正在检索知识库…</p>
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
					<p class="mt-6 text-sm text-zinc-400">正在识别…</p>
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
					<svg
						xmlns="http://www.w3.org/2000/svg"
						fill="none"
						viewBox="0 0 24 24"
						stroke-width="1.7"
						stroke="currentColor"
						class="h-4 w-4 animate-pulse text-emerald-400"
						aria-hidden="true"
					>
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							d="M12 18.75a6 6 0 0 0 6-6v-1.5m-6 7.5a6 6 0 0 1-6-6v-1.5m6 7.5v3.75m-3.75 0h7.5M12 15.75a3 3 0 0 1-3-3V4.5a3 3 0 1 1 6 0v8.25a3 3 0 0 1-3 3Z"
						/>
					</svg>
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
