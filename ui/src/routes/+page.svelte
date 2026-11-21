<script lang="ts">
	import { tick } from 'svelte';
	import { fade, fly } from 'svelte/transition';
	import { askQuestion, citationHref, type ChatAnswer } from '../lib/api/chat.js';
	import ChatPanel from '#lib/components/ChatPanel.svelte';
	import JellyOrb from '#lib/components/JellyOrb.svelte';
	import MeltBackground from '#lib/components/MeltBackground.svelte';
	import WaveBackground from '#lib/components/WaveBackground.svelte';

	type Turn = { question: string; result?: ChatAnswer; error?: string };

	const chatColors = ['#050807', '#07120e', '#08131a', '#0b0f08'];
	let listening = $state(false);
	let pending = $state(false);
	let conversation = $state<Turn[]>([]);
	let referenceIndex = $state(0);
	let conversationEl: HTMLDivElement | undefined = $state();
	const inChat = $derived(conversation.length > 0);
	const references = $derived(conversation[referenceIndex]?.result?.citations ?? []);
	const orbClass = $derived(
		inChat
			? 'left-6 top-6 h-16 w-16'
			: 'left-1/2 top-[28vh] h-64 w-64 -translate-x-1/2 -translate-y-1/2 sm:h-80 sm:w-80 ' +
					(listening ? 'scale-110' : 'scale-100')
	);

	async function ask(question: string) {
		if (pending) return;
		const index = conversation.length;
		conversation = [...conversation, { question }];
		referenceIndex = index;
		pending = true;
		await tick();
		conversationEl?.scrollTo({ top: conversationEl.scrollHeight });

		try {
			const result = await askQuestion(question);
			conversation[index] = { question, result };
		} catch (failure) {
			conversation[index] = {
				question,
				error: failure instanceof Error ? failure.message : '问答失败，请重试。'
			};
		} finally {
			pending = false;
			await tick();
			conversationEl?.scrollTo({ top: conversationEl.scrollHeight });
		}
	}
</script>

<svelte:head>
	<title>RAGanswer</title>
</svelte:head>

<main class="relative min-h-dvh w-full overflow-x-hidden bg-[#050706] text-zinc-100">
	<div
		class="pointer-events-none fixed inset-0 z-0 transition-all duration-1000 ease-out motion-reduce:transition-none"
		class:opacity-100={inChat}
		class:opacity-0={!inChat}
		style:background-color={chatColors[(conversation.length + 3) % chatColors.length]}
		aria-hidden="true"
	></div>
	<div class="pointer-events-none fixed inset-0 z-0 opacity-[0.14]" aria-hidden="true">
		<MeltBackground revealed={true} />
	</div>
	<WaveBackground visible={!inChat} />
	<div
		class="pointer-events-none fixed inset-0 z-[1] transition-opacity duration-[1600ms] ease-out motion-reduce:transition-none"
		class:opacity-100={listening && !inChat}
		class:opacity-0={!listening || inChat}
		style="background: radial-gradient(ellipse at 50% 45%, rgba(3, 6, 5, 0.55), rgba(3, 6, 5, 0.12) 62%, transparent 82%);"
		aria-hidden="true"
	></div>
	<div
		class={'pointer-events-none fixed z-20 transition-all duration-1000 ease-in-out motion-reduce:transition-none ' +
			orbClass}
	>
		<JellyOrb />
	</div>

	{#if inChat}
		<h1 class="pointer-events-none fixed top-10 left-24 z-20 text-sm font-medium text-zinc-200">
			工厂知识问答
		</h1>

		<div
			class="relative z-10 flex min-h-dvh flex-col lg:h-dvh lg:flex-row"
			in:fade={{ duration: 450 }}
		>
			<section class="flex min-h-0 flex-1 flex-col" aria-label="问答对话">
				<div
					bind:this={conversationEl}
					class="min-h-0 flex-1 overflow-y-auto px-5 pt-24 sm:px-10 lg:pl-24"
					aria-live="polite"
				>
					<div class="mx-auto w-full max-w-2xl space-y-9 pb-6">
						{#each conversation as turn, i (i + ':' + turn.question)}
							<article class="space-y-5">
								<div class="flex flex-col items-end">
									<span class="text-xs text-zinc-400">你</span>
									<p class="mt-2 max-w-xl text-right text-lg leading-relaxed text-zinc-100">
										{turn.question}
									</p>
								</div>
								{#if turn.result}
									<div class="border-l border-emerald-400/30 pl-5" in:fade={{ duration: 300 }}>
										<p class="text-xs font-medium text-emerald-300">回答</p>
										<p class="mt-3 text-base leading-7 whitespace-pre-wrap text-zinc-200">
											{turn.result.answer}
										</p>
										{#if turn.result.citations.length}
											<button
												type="button"
												class="mt-3 rounded-md px-2 py-1 text-sm text-emerald-300 underline focus-visible:outline-2"
												aria-controls="references"
												aria-pressed={referenceIndex === i}
												onclick={() => (referenceIndex = i)}
											>
												查看出处（{turn.result.citations.length}）
											</button>
										{/if}
									</div>
								{:else if turn.error}
									<p role="alert" class="text-sm text-rose-200">{turn.error}</p>
								{/if}
							</article>
						{/each}
						{#if pending}
							<p role="status" class="border-l border-emerald-400/30 pl-5 text-sm text-emerald-300">
								正在检索知识库…
							</p>
						{/if}
					</div>
				</div>

				<div class="shrink-0 px-5 pb-6 sm:px-10 lg:pl-24">
					<div class="mx-auto max-w-2xl"><ChatPanel bind:listening {pending} onask={ask} /></div>
				</div>
			</section>

			<aside
				id="references"
				class="min-h-0 w-full overflow-y-auto border-t border-white/10 bg-[#070b09]/80 px-5 pt-6 pb-10 backdrop-blur-xl lg:h-full lg:w-80 lg:border-t-0 lg:border-l"
				in:fly={{ x: 80, duration: 700, delay: 120 }}
				aria-label="参考文献"
			>
				<h2 class="text-sm font-medium text-zinc-200">参考文献</h2>
				{#if references.length}
					<ol class="mt-6 space-y-6">
						{#each references as reference, i (reference.document_id + ':' + i)}
							{@const href = citationHref(reference.url)}
							{@const location = [
								reference.version,
								reference.section,
								reference.page === null ? null : '第 ' + reference.page + ' 页'
							]
								.filter(Boolean)
								.join(' · ')}
							<li class="border-l border-white/10 pl-4">
								<p class="text-sm leading-6 font-medium text-emerald-200">
									[{i + 1}] {reference.title}
								</p>
								{#if location}<p class="mt-1 text-xs text-zinc-400">{location}</p>{/if}
								<blockquote class="mt-2 text-sm leading-6 text-zinc-300">
									{reference.excerpt}
								</blockquote>
								{#if href}<a
										{href}
										rel="external"
										class="mt-2 inline-block text-sm text-emerald-300 underline"
									>
										打开原文
									</a>{/if}
							</li>
						{/each}
					</ol>
				{:else}
					<p class="mt-4 text-sm text-zinc-400">
						{pending ? '正在检索知识库…' : '这条回答没有出处。'}
					</p>
				{/if}
			</aside>
		</div>
	{:else}
		<section class="relative z-10 mx-auto flex w-full max-w-2xl flex-col px-6 pt-[48dvh] pb-8">
			<h1 class="text-center text-3xl font-medium tracking-tight text-zinc-50 sm:text-4xl">
				工厂知识问答
			</h1>
			<ChatPanel bind:listening onask={ask} />
		</section>
	{/if}
</main>
