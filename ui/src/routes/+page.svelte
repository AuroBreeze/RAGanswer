<script lang="ts">
	import ChatPanel from '#lib/components/ChatPanel.svelte';
	import JellyOrb from '#lib/components/JellyOrb.svelte';
	import MeltBackground from '#lib/components/MeltBackground.svelte';
	import WaveBackground from '#lib/components/WaveBackground.svelte';

	let listening = $state(false);
	let revealed = $state(false);

	$effect(() => {
		if (listening) revealed = true;
	});
</script>

<svelte:head>
	<title>RAGanswer</title>
</svelte:head>

<main class="relative flex min-h-dvh w-full flex-col bg-[#050706] text-zinc-100">
	<WaveBackground />
	<MeltBackground {revealed} />
	<div
		class={`pointer-events-none fixed inset-0 z-[1] transition-opacity duration-[1600ms] ease-out motion-reduce:transition-none ${revealed ? 'opacity-100' : 'opacity-0'}`}
		style="background: radial-gradient(ellipse at 50% 45%, rgba(3, 6, 5, 0.55), rgba(3, 6, 5, 0.12) 62%, transparent 82%);"
		aria-hidden="true"
	></div>

	<div
		class="relative z-10 flex flex-1 flex-col items-center justify-start px-4 pt-40 pb-10 sm:justify-center sm:pt-44"
	>
		<section class="relative w-full max-w-2xl px-6 pt-36 pb-8 sm:px-10 sm:pt-40 sm:pb-10">
			<div
				class={`pointer-events-none absolute top-0 left-1/2 z-10 h-72 w-72 transition-transform duration-700 ease-out motion-reduce:transition-none sm:h-80 sm:w-80 ${
					listening
						? '-translate-x-[64%] -translate-y-[44%] scale-125'
						: '-translate-x-1/2 -translate-y-1/2 scale-100'
				}`}
			>
				<JellyOrb />
			</div>

			<h1 class="text-center text-3xl font-medium tracking-tight text-zinc-50 sm:text-4xl">
				工厂知识问答
			</h1>
			<ChatPanel bind:listening />
		</section>
	</div>
</main>
