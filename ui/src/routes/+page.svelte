<script lang="ts">
	import ChatPanel from '#lib/components/ChatPanel.svelte';
	import JellyOrb from '#lib/components/JellyOrb.svelte';
</script>

<svelte:head>
	<title>RAGanswer</title>
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

	<div
		class="relative z-10 flex flex-1 flex-col items-center justify-start px-4 pt-40 pb-10 sm:justify-center sm:pt-44"
	>
		<section
			class="relative w-full max-w-2xl rounded-[2rem] border border-white/10 bg-[#0a0f0c]/80 px-6 pt-36 pb-8 shadow-2xl shadow-black/40 backdrop-blur-xl sm:px-10 sm:pt-40 sm:pb-10"
		>
			<div
				class="pointer-events-none absolute top-0 left-1/2 z-10 h-72 w-72 -translate-x-1/2 -translate-y-1/2 sm:h-80 sm:w-80"
			>
				<JellyOrb />
			</div>

			<h1 class="text-center text-3xl font-medium tracking-tight text-zinc-50 sm:text-4xl">
				工厂知识问答
			</h1>

			<ChatPanel />
		</section>

		<p class="mt-8 text-center text-[11px] text-zinc-600">Orb 可能会出错，请核对重要信息。</p>
	</div>
</main>
