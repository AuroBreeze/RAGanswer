<script lang="ts">
	import { onDestroy } from 'svelte';
	import { askQuestion, citationHref, type ChatAnswer } from '../api/chat.js';

	interface SpeechInput {
		lang: string;
		onresult: ((event: { results: ArrayLike<ArrayLike<{ transcript: string }>> }) => void) | null;
		onerror: ((event: { error: string }) => void) | null;
		onend: (() => void) | null;
		start(): void;
		stop(): void;
		abort(): void;
	}

	let { listening = $bindable(false) }: { listening?: boolean } = $props();
	let question = $state('');
	let pending = $state(false);
	let error = $state('');
	let voiceError = $state('');
	let result = $state<ChatAnswer | null>(null);
	let speech: SpeechInput | undefined;

	async function ask(event: SubmitEvent) {
		event.preventDefault();
		const query = question.trim();
		if (!query || pending || listening) return;
		pending = true;
		error = '';
		result = null;
		try {
			result = await askQuestion(query);
		} catch (failure) {
			error = failure instanceof Error ? failure.message : '问答失败，请重试。';
		} finally {
			pending = false;
		}
	}

	function toggleListening() {
		if (listening) {
			speech?.stop();
			return;
		}
		voiceError = '';
		const browser = window as Window & {
			SpeechRecognition?: new () => SpeechInput;
			webkitSpeechRecognition?: new () => SpeechInput;
		};
		const Recognition = browser.SpeechRecognition ?? browser.webkitSpeechRecognition;
		if (!Recognition) {
			voiceError = '当前浏览器不支持语音输入，可以直接键入问题。';
			return;
		}
		const input = new Recognition();
		input.lang = 'zh-CN';
		input.onresult = ({ results }) => {
			const text = results[0]?.[0]?.transcript.trim();
			if (text) question = text.slice(0, 4000);
		};
		input.onerror = ({ error }) => {
			if (error === 'aborted') return;
			voiceError = error === 'not-allowed' ? '未获得麦克风权限。' : '语音识别失败，请重试。';
		};
		input.onend = () => {
			listening = false;
			speech = undefined;
		};
		try {
			input.start();
			speech = input;
			listening = true;
		} catch {
			voiceError = '无法启动语音识别。';
		}
	}

	onDestroy(() => speech?.abort());
</script>

<form class="mt-6 space-y-3" onsubmit={ask} aria-busy={pending}>
	<label for="question" class="block text-sm font-medium text-zinc-300">你的问题</label>
	<textarea
		id="question"
		bind:value={question}
		maxlength="4000"
		rows="3"
		placeholder="例如，设备点检记录表在哪里领取？"
		class="w-full resize-y rounded-2xl border border-white/15 bg-black/25 px-4 py-3 text-sm text-zinc-100 placeholder:text-zinc-500 focus:border-emerald-400 focus:outline-none"
	></textarea>
	<div class="flex flex-wrap gap-3">
		<button
			type="button"
			onclick={toggleListening}
			aria-pressed={listening}
			disabled={pending}
			class="min-h-11 rounded-xl border border-white/15 px-4 text-sm text-zinc-200 hover:border-emerald-400 focus-visible:outline-2 focus-visible:outline-emerald-400 disabled:opacity-50"
		>
			{listening ? '结束聆听' : '点击说话'}
		</button>
		<button
			type="submit"
			disabled={!question.trim() || pending || listening}
			class="min-h-11 rounded-xl bg-emerald-300 px-5 text-sm font-medium text-emerald-950 hover:bg-emerald-200 focus-visible:outline-2 focus-visible:outline-emerald-400 disabled:opacity-50"
		>
			提交问题
		</button>
	</div>
</form>

{#if voiceError}<p class="mt-3 text-sm text-amber-200" role="status">{voiceError}</p>{/if}
{#if error}<p class="mt-3 text-sm text-rose-200" role="alert">{error}</p>{/if}
{#if pending}<p class="mt-4 text-sm text-zinc-300" role="status">正在查询知识库…</p>{/if}

{#if result}
	<section
		aria-label="问答结果"
		aria-live="polite"
		class="mt-6 space-y-4 border-t border-white/10 pt-5"
	>
		<h2 class="text-lg font-medium text-zinc-100">回答</h2>
		<p class="text-sm leading-7 whitespace-pre-wrap text-zinc-200">{result.answer}</p>
		{#if result.citations.length}
			<h3 class="text-sm font-medium text-zinc-300">出处</h3>
			<ol class="space-y-3">
				{#each result.citations as citation, index}
					{@const href = citationHref(citation.url)}
					<li class="rounded-xl border border-white/10 px-4 py-3 text-sm">
						<p class="font-medium text-emerald-200">[{index + 1}] {citation.title}</p>
						<p class="mt-1 text-xs text-zinc-400">
							{[
								citation.version,
								citation.section,
								citation.page === null ? null : `第 ${citation.page} 页`
							]
								.filter(Boolean)
								.join(' · ')}
						</p>
						<blockquote class="mt-2 leading-6 text-zinc-300">{citation.excerpt}</blockquote>
						{#if href}<a class="mt-2 inline-block text-emerald-300 underline" {href}>
								打开原文
							</a>{/if}
					</li>
				{/each}
			</ol>
		{/if}
	</section>
{/if}
