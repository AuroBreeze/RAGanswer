<script lang="ts">
	import { onMount } from 'svelte';
	import AdminIcon from '#lib/components/AdminIcon.svelte';
	import {
		approveArtifact,
		listDocuments,
		revokeDocument,
		uploadArtifact,
		type KnowledgeDocument
	} from '../../lib/api/admin.js';

	const MAX_SIZE = 10 * 1024 * 1024;
	let token = $state('');
	let documents = $state<KnowledgeDocument[]>([]);
	let connected = $state(false);
	let busy = $state(false);
	let refreshing = $state(false);
	let error = $state('');
	let notice = $state('');
	let view = $state<'list' | 'grid'>('list');
	let sort = $state(false);
	let dragOver = $state(false);
	const entries = $derived(
		sort ? [...documents].sort((a, b) => a.title.localeCompare(b.title, 'zh-CN')) : documents
	);

	function status(doc: KnowledgeDocument) {
		if (doc.document_status === 'revoked' || doc.source_status === 'revoked') return '已停用';
		if (doc.error?.code === 'SOURCE_NOT_APPROVED') return '待复核';
		if (doc.job_status === 'failed') return '入库失败';
		if (doc.job_status === 'queued' || doc.job_status === 'indexing') return '正在入库';
		if (doc.searchable) return '可检索';
		return doc.job_status === 'ready' ? '已索引' : '未入库';
	}

	function failure(reason: unknown) {
		return reason instanceof Error ? reason.message : '管理请求失败。';
	}

	async function refresh() {
		if (refreshing || !token.trim()) return;
		const activeToken = token.trim();
		refreshing = true;
		try {
			const result = await listDocuments(activeToken);
			if (activeToken !== token.trim()) return;
			documents = result;
			connected = true;
			error = '';
		} catch (reason) {
			if (activeToken !== token.trim()) return;
			documents = [];
			connected = false;
			error = failure(reason);
		} finally {
			refreshing = false;
		}
	}

	function resetConnection() {
		connected = false;
		documents = [];
		error = '';
		notice = '';
	}

	async function act(action: () => Promise<void>, success: string) {
		if (busy || !connected) return;
		busy = true;
		error = '';
		notice = '';
		try {
			await action();
			await refresh();
			if (connected) notice = success;
		} catch (reason) {
			error = failure(reason);
		} finally {
			busy = false;
		}
	}

	async function importFiles(files: FileList | null) {
		if (!files?.length) return;
		const selected = Array.from(files);
		if (
			selected.some((file) => !file.name.toLowerCase().endsWith('.ndjson') || file.size > MAX_SIZE)
		) {
			error = '请选择不超过 10 MiB 的 NDJSON 解析包。';
			return;
		}
		await act(async () => {
			for (const file of selected) await uploadArtifact(token.trim(), file);
		}, '已提交入库，请查看任务状态。');
	}

	async function choose(event: Event) {
		const element = event.currentTarget as HTMLInputElement;
		await importFiles(element.files);
		element.value = '';
	}

	async function drop(event: DragEvent) {
		event.preventDefault();
		dragOver = false;
		await importFiles(event.dataTransfer?.files ?? null);
	}

	async function approve(parseId: string) {
		if (!window.confirm('确认你已核对这份资料的原文，并允许部分解析结果入库？')) return;
		await act(() => approveArtifact(token.trim(), parseId), '复核已确认，正在重试入库。');
	}

	async function revoke(doc: KnowledgeDocument) {
		if (!window.confirm('停用「' + doc.title + '」？问答将不再引用这份资料。')) return;
		await act(() => revokeDocument(token.trim(), doc.document_id), '资料已停用。');
	}

	onMount(() => {
		const timer = setInterval(() => {
			if (
				!busy &&
				connected &&
				documents.some((doc) => doc.job_status === 'queued' || doc.job_status === 'indexing')
			) {
				void refresh();
			}
		}, 2000);
		return () => clearInterval(timer);
	});
</script>

<svelte:head><title>知识库管理 · RAGanswer</title></svelte:head>

<h1 class="text-xl font-semibold tracking-tight text-zinc-900">知识库</h1>
<p class="mt-2 text-sm text-zinc-600">
	导入网页或 PDF 的 NDJSON 解析包，查看索引状态和可检索资料。
</p>

<form
	class="mt-5 flex flex-wrap items-end gap-3 rounded-xl border border-zinc-200 bg-white p-4"
	onsubmit={(event) => {
		event.preventDefault();
		void refresh();
	}}
>
	<label class="min-w-56 flex-1 text-sm font-medium text-zinc-700" for="admin-token">
		管理令牌
		<input
			id="admin-token"
			type="password"
			autocomplete="off"
			bind:value={token}
			oninput={resetConnection}
			disabled={busy}
			class="mt-1.5 block min-h-10 w-full rounded-md border border-zinc-300 px-3"
		/>
	</label>
	<button
		type="submit"
		disabled={!token.trim() || busy || refreshing}
		class="min-h-10 rounded-lg bg-zinc-800 px-4 text-sm text-white disabled:opacity-50"
	>
		{connected ? '刷新资料' : '连接知识库'}
	</button>
</form>

{#if error}<p role="alert" class="mt-3 text-sm text-red-700">{error}</p>{/if}
{#if notice}<p role="status" class="mt-3 text-sm text-emerald-700">{notice}</p>{/if}
{#if connected}
	<input
		id="knowledge-upload"
		type="file"
		class="sr-only"
		accept=".ndjson"
		multiple
		onchange={choose}
		disabled={busy}
	/>
	<label
		for="knowledge-upload"
		ondragover={(event) => {
			event.preventDefault();
			dragOver = true;
		}}
		ondragleave={() => (dragOver = false)}
		ondrop={drop}
		class="mt-5 block cursor-pointer rounded-2xl border border-dashed border-zinc-300 bg-white px-6 py-9 text-center hover:bg-zinc-50"
		class:bg-zinc-50={dragOver}
	>
		<span class="mx-auto grid h-11 w-11 place-items-center rounded-full bg-zinc-100 text-zinc-600">
			<AdminIcon name="upload" class="h-5 w-5" />
		</span>
		<span class="mt-3 block text-sm font-medium text-zinc-800">
			{busy ? '正在处理…' : '点击或拖入解析包'}
		</span>
		<span class="mt-1 block text-xs text-zinc-500">.ndjson，每个文件最多 10 MiB</span>
	</label>

	<div class="mt-8 flex flex-wrap items-center justify-between gap-3">
		<h2 class="font-medium text-zinc-900">
			已登记资料 <span class="ml-1 text-sm text-zinc-500">{documents.length}</span>
		</h2>
		<div class="flex items-center gap-3">
			<button
				type="button"
				onclick={() => (sort = !sort)}
				aria-pressed={sort}
				class="rounded-md px-2 py-2 text-sm text-zinc-700"
			>
				{sort ? '按标题排序' : '按更新时间排序'}
			</button>
			<button
				type="button"
				onclick={() => (view = view === 'list' ? 'grid' : 'list')}
				aria-label={view === 'list' ? '切换为网格视图' : '切换为列表视图'}
				class="grid h-9 w-9 place-items-center rounded-lg border border-zinc-200 bg-white text-zinc-600"
			>
				<AdminIcon name={view === 'list' ? 'layout-grid' : 'list-bullet'} class="h-4 w-4" />
			</button>
		</div>
	</div>

	{#if entries.length === 0}
		<p
			class="mt-3 rounded-xl border border-dashed border-zinc-300 bg-white py-10 text-center text-sm text-zinc-600"
		>
			暂无已登记资料。
		</p>
	{:else}
		<ul class={view === 'grid' ? 'mt-3 grid gap-3 sm:grid-cols-2' : 'mt-3 space-y-2'}>
			{#each entries as doc (doc.document_id)}
				<li class="rounded-xl border border-zinc-200 bg-white p-4">
					<div class="flex min-w-0 flex-wrap items-center gap-3">
						<span
							class="grid h-9 w-9 shrink-0 place-items-center rounded-lg bg-emerald-50 text-emerald-700"
						>
							<AdminIcon name="document" class="h-4 w-4" />
						</span>
						<div class="min-w-0 flex-1">
							<p class="text-sm font-medium break-words text-zinc-900">{doc.title}</p>
							<p class="mt-1 text-xs text-zinc-500">
								{doc.source_type === 'pdf' ? 'PDF' : '网页'} · {doc.indexed_chunk_count ?? 0} 个片段
								{#if doc.version}
									· {doc.version}{/if}
							</p>
						</div>
						<span
							class="rounded-full bg-zinc-100 px-2.5 py-1 text-xs font-medium text-zinc-700"
							class:text-emerald-700={doc.searchable}
						>
							{status(doc)}
						</span>
					</div>
					{#if doc.error}<p class="mt-2 text-xs text-red-700">{doc.error.message}</p>{/if}
					<div class="mt-2 flex gap-4">
						{#if doc.parse_id && doc.error?.code === 'SOURCE_NOT_APPROVED' && doc.document_status === 'active'}
							<button
								type="button"
								disabled={busy}
								onclick={() => {
									if (doc.parse_id) void approve(doc.parse_id);
								}}
								class="text-sm text-emerald-700 underline"
							>
								复核通过
							</button>
						{/if}
						{#if doc.document_status === 'active'}
							<button
								type="button"
								disabled={busy}
								onclick={() => void revoke(doc)}
								class="ml-auto text-xs text-zinc-600 underline"
							>
								停用
							</button>
						{/if}
					</div>
				</li>
			{/each}
		</ul>
	{/if}
{/if}
