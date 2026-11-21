<script lang="ts">
	import { onMount } from 'svelte';
	import { fade } from 'svelte/transition';
	import JellyOrb from '#lib/components/JellyOrb.svelte';

	const prompts = [
		'帮我总结一段文字',
		'解释一个技术概念',
		'写一个 Svelte 组件',
		'给我一些产品灵感'
	];

	const categories = ['全部', '写作', '代码', '分析', '灵感'];

	let index = $state(0);
	let listening = $state(false);

	function toggleListening() {
		listening = !listening;
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
	<div class="pointer-events-none absolute inset-0 overflow-hidden" aria-hidden="true">
		<div
			class="absolute inset-0"
			style="background:
				radial-gradient(ellipse at 50% -8%, rgba(16, 185, 129, 0.20), transparent 55%),
				radial-gradient(ellipse at 85% 100%, rgba(6, 78, 59, 0.16), transparent 52%),
				linear-gradient(180deg, #070b09 0%, #050706 55%, #030504 100%);"
		></div>
	</div>

	<header class="relative z-10 flex items-center justify-between px-5 py-5 sm:px-10">
		<div class="flex items-center gap-2 text-sm text-zinc-400">
			<button
				type="button"
				class="grid h-8 w-8 place-items-center rounded-lg text-zinc-500 transition hover:bg-white/5 hover:text-zinc-300"
				aria-label="返回"
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
					<path stroke-linecap="round" stroke-linejoin="round" d="M15.75 19.5 8.25 12l7.5-7.5" />
				</svg>
			</button>
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

			<button
				type="button"
				class={`mt-6 flex h-14 w-full items-center justify-center gap-3 rounded-2xl px-6 text-sm font-medium shadow-2xl shadow-black/30 transition ${
					listening
						? 'bg-emerald-400 text-emerald-950 hover:bg-emerald-300'
						: 'bg-white text-zinc-900 hover:bg-zinc-100'
				}`}
				onclick={toggleListening}
				aria-pressed={listening}
			>
				<svg
					xmlns="http://www.w3.org/2000/svg"
					fill="none"
					viewBox="0 0 24 24"
					stroke-width="1.7"
					stroke="currentColor"
					class="h-5 w-5"
					aria-hidden="true"
				>
					<path
						stroke-linecap="round"
						stroke-linejoin="round"
						d="M12 18.75a6 6 0 0 0 6-6v-1.5m-6 7.5a6 6 0 0 1-6-6v-1.5m6 7.5v3.75m-3.75 0h7.5M12 15.75a3 3 0 0 1-3-3V4.5a3 3 0 1 1 6 0v8.25a3 3 0 0 1-3 3Z"
					/>
				</svg>

				<span>{listening ? '正在聆听，点击结束' : '点击说话'}</span>

				{#if listening}
					<span class="flex items-center gap-1" aria-hidden="true">
						<span class="h-1.5 w-1.5 animate-pulse rounded-full bg-emerald-950/70"></span>
						<span
							class="h-2.5 w-1.5 animate-pulse rounded-full bg-emerald-950/70"
							style="animation-delay: 120ms;"
						></span>
						<span
							class="h-1.5 w-1.5 animate-pulse rounded-full bg-emerald-950/70"
							style="animation-delay: 240ms;"
						></span>
					</span>
				{/if}
			</button>
		</section>

		<p class="mt-8 text-center text-[11px] text-zinc-600">
			Orb 可能会出错，请核对重要信息。
		</p>
	</div>
</main>
