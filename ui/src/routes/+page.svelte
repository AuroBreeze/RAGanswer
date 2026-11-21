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

	const categories = ['全部', '写作', '代码', '分析', '灵感'];

	let index = $state(0);

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
	<WaveBackground />

	<header class="relative z-10 flex items-center justify-between px-5 py-5 sm:px-10">
		<div class="flex items-center gap-2 text-sm text-zinc-400">
			<span class="text-zinc-300">New chat</span>
			<span
				class="rounded-full border border-emerald-400/20 bg-emerald-400/10 px-2.5 py-1 text-[10px] font-medium tracking-wide text-emerald-300"
			>
				Orb 1.0
			</span>
		</div>

		<div class="flex items-center gap-1">
			<button
				type="button"
				class="grid h-9 w-9 place-items-center rounded-lg text-zinc-500 transition hover:bg-white/5 hover:text-zinc-300"
				aria-label="收藏"
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
						d="M17.25 4.5H6.75A1.75 1.75 0 0 0 5 6.25v13.5l7-4.2 7 4.2V6.25a1.75 1.75 0 0 0-1.75-1.75Z"
					/>
				</svg>
			</button>
			<button
				type="button"
				class="grid h-9 w-9 place-items-center rounded-lg text-zinc-500 transition hover:bg-white/5 hover:text-zinc-300"
				aria-label="分享"
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
						d="M9 12.75 15 9m-6 3.75 6 3M6.75 20.25h10.5a2.25 2.25 0 0 0 2.25-2.25V6a2.25 2.25 0 0 0-2.25-2.25H6.75A2.25 2.25 0 0 0 4.5 6v12a2.25 2.25 0 0 0 2.25 2.25Z"
					/>
				</svg>
			</button>
		</div>
	</header>

	<div class="relative z-10 flex flex-1 flex-col items-center justify-start px-4 pb-10 pt-40 sm:justify-center sm:pt-44">
		<section
			class="relative w-full max-w-2xl rounded-[2rem] border border-white/10 bg-[#0a0f0c]/80 px-6 pb-8 pt-36 shadow-2xl shadow-black/40 backdrop-blur-xl sm:px-10 sm:pb-10 sm:pt-40"
		>
			<div
				class="pointer-events-none absolute left-1/2 top-0 z-10 h-72 w-72 -translate-x-1/2 -translate-y-1/2 sm:h-80 sm:w-80"
			>
				<JellyOrb />
			</div>

			<h1 class="text-center text-3xl font-medium tracking-tight text-zinc-50 sm:text-4xl">
				你可以问…
			</h1>

			<div class="mt-4 flex min-h-7 items-center justify-center text-base text-zinc-300 sm:text-lg">
				{#key index}
					<span in:fade={{ duration: 240 }} class="text-center">{prompts[index]}</span>
				{/key}
			</div>

			<div class="mt-8 flex flex-wrap items-center justify-center gap-x-6 gap-y-3 text-sm">
				{#each categories as category, i}
					<button
						type="button"
						class={i === 0
							? 'border-b-2 border-emerald-400 pb-1 text-emerald-300'
							: 'border-b-2 border-transparent pb-1 text-zinc-500 transition hover:text-zinc-300'}
					>
						{category}
					</button>
				{/each}
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
		</section>

		<p class="mt-8 text-center text-[11px] text-zinc-600">
			Orb 可能会出错，请核对重要信息。
		</p>
	</div>
</main>
