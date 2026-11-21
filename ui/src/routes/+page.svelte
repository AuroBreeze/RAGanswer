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

			<form
				class="mt-6 flex w-full items-center gap-2 rounded-2xl bg-white p-2 pl-3 shadow-2xl shadow-black/30"
				onsubmit={(event) => event.preventDefault()}
			>
				<span
					class="h-6 w-6 shrink-0 rounded-full"
					style="background: radial-gradient(circle at 30% 30%, #d1fae5, #34d399 40%, #065f46 75%, #022c22);"
					aria-hidden="true"
				></span>

				<label class="sr-only" for="prompt">输入你的问题</label>
				<input
					id="prompt"
					name="prompt"
					type="text"
					placeholder="输入你的问题..."
					autocomplete="off"
					class="min-w-0 flex-1 bg-transparent px-1 text-sm text-zinc-900 placeholder:text-zinc-400 focus:outline-none"
				/>

				<button
					type="button"
					class="grid h-9 w-9 place-items-center rounded-xl text-zinc-400 transition hover:bg-zinc-100 hover:text-zinc-700"
					aria-label="语音输入"
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
				</button>

				<button
					type="submit"
					class="grid h-9 w-9 place-items-center rounded-xl bg-emerald-400 text-emerald-950 transition hover:bg-emerald-300"
					aria-label="发送"
				>
					<svg
						xmlns="http://www.w3.org/2000/svg"
						fill="none"
						viewBox="0 0 24 24"
						stroke-width="2"
						stroke="currentColor"
						class="h-4 w-4"
						aria-hidden="true"
					>
						<path stroke-linecap="round" stroke-linejoin="round" d="M4.5 12h15m0 0-6-6m6 6-6 6" />
					</svg>
				</button>
			</form>
		</section>

		<p class="mt-8 text-center text-[11px] text-zinc-600">
			Orb 可能会出错，请核对重要信息。
		</p>
	</div>
</main>
