<script lang="ts">
	import { onMount } from 'svelte';
	import { fade } from 'svelte/transition';
	import JellyOrb from '#lib/components/JellyOrb.svelte';
	import WaveBackground from '#lib/components/WaveBackground.svelte';

	const prompts = [
		'帮我总结一段文字',
		'解释一个技术概念',
		'写一个 Svelte 组件',
		'给我一些产品灵感'
	];

	const simulatedSpeech = [
		'帮我查一下三号线的设备维护记录。',
		'这个报警代码是什么意思？',
		'把今天的巡检结果整理成报告。',
		'上次更换轴承是什么时候？'
	];

	let index = $state(0);
	let active = $state(false);
	let speechIndex = $state(-1);
	let speech = $state('');

	function handleScreenTap() {
		speechIndex = (speechIndex + 1) % simulatedSpeech.length;
		speech = simulatedSpeech[speechIndex];
		active = true;
	}

	onMount(() => {
		const timer = setInterval(() => {
			index = (index + 1) % prompts.length;
		}, 3200);

		return () => clearInterval(timer);
	});
</script>

<svelte:head>
	<title>Orb Chat</title>
</svelte:head>

<main class="relative flex min-h-dvh w-full flex-col bg-[#050706] text-zinc-100">
	<WaveBackground ontap={handleScreenTap} />

	<div class="relative z-10 flex flex-1 flex-col items-center justify-start px-4 pb-10 pt-40 sm:justify-center sm:pt-44">
		<section
			class={`relative w-full max-w-2xl px-6 pb-8 transition-all duration-700 ease-out sm:px-10 sm:pb-10 ${
				active ? 'pt-48 sm:pt-52' : 'pt-36 sm:pt-40'
			}`}
		>
			<div
				class={`pointer-events-none absolute left-1/2 top-0 z-10 h-72 w-72 transition-transform duration-700 ease-out sm:h-80 sm:w-80 ${
					active
						? '-translate-x-[64%] -translate-y-[44%] scale-125'
						: '-translate-x-1/2 -translate-y-1/2 scale-100'
				}`}
			>
				<JellyOrb />
			</div>

			{#if active}
				<div class="text-center" in:fade={{ duration: 300 }}>
					<p class="text-[11px] uppercase tracking-[0.28em] text-emerald-400/70">你说</p>
					{#key speechIndex}
						<p
							in:fade={{ duration: 240 }}
							class="mt-3 text-2xl font-medium leading-snug tracking-tight text-zinc-50 sm:text-3xl"
						>
							{speech}
						</p>
					{/key}
				</div>
			{:else}
				<h1 class="text-center text-3xl font-medium tracking-tight text-zinc-50 sm:text-4xl">
					你可以问…
				</h1>

				<div class="mt-4 flex min-h-7 items-center justify-center text-base text-zinc-300 sm:text-lg">
					{#key index}
						<span in:fade={{ duration: 240 }} class="text-center">{prompts[index]}</span>
					{/key}
				</div>
			{/if}

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
				{active ? '再次触摸，继续说话' : '触摸屏幕来输入语音'}
			</p>
		</section>

		<p class="mt-8 text-center text-[11px] text-zinc-600">
			Orb 可能会出错，请核对重要信息。
		</p>
	</div>
</main>
