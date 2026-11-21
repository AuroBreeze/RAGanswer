<script lang="ts">
	import { onMount } from 'svelte';
	import AdminIcon from '#lib/components/AdminIcon.svelte';

	type FileType = 'pdf' | 'md' | 'docx' | 'txt';
	type Status = 'indexed' | 'processing' | 'failed';

	type KnowledgeFile = {
		id: number;
		name: string;
		size: string;
		added: string;
		type: FileType;
		status: Status;
	};

	const ALLOWED: FileType[] = ['pdf', 'txt', 'md', 'docx'];
	const MAX_SIZE = 10 * 1024 * 1024;

	const typeStyles: Record<FileType, string> = {
		pdf: 'bg-blue-50 text-blue-600',
		md: 'bg-orange-50 text-orange-500',
		docx: 'bg-emerald-50 text-emerald-600',
		txt: 'bg-zinc-100 text-zinc-500'
	};

	const statusMeta: Record<Status, { label: string; badge: string }> = {
		indexed: { label: 'Indexed', badge: 'bg-emerald-50 text-emerald-700' },
		processing: { label: 'Processing', badge: 'bg-amber-50 text-amber-700' },
		failed: { label: 'Failed', badge: 'bg-red-50 text-red-600' }
	};

	let nextId = 5;
	let files = $state<KnowledgeFile[]>([
		{
			id: 1,
			name: 'Product_Specs.pdf',
			size: '2.4 MB',
			added: '今天',
			type: 'pdf',
			status: 'indexed'
		},
		{
			id: 2,
			name: 'Knowledge_Q1.md',
			size: '14 KB',
			added: '昨天',
			type: 'md',
			status: 'processing'
		},
		{
			id: 3,
			name: '客服话术库.docx',
			size: '180 KB',
			added: '2 天前',
			type: 'docx',
			status: 'indexed'
		},
		{
			id: 4,
			name: 'API_Reference.txt',
			size: '36 KB',
			added: '3 天前',
			type: 'txt',
			status: 'failed'
		}
	]);

	let view = $state<'list' | 'grid'>('list');
	let dragOver = $state(false);
	let error = $state('');
	let sortDir = $state<'none' | 'asc' | 'desc'>('none');
	let inputEl = $state<HTMLInputElement>();
	let timers: ReturnType<typeof setTimeout>[] = [];
	let errorTimer: ReturnType<typeof setTimeout> | undefined;

	const sortedFiles = $derived.by(() => {
		if (sortDir === 'none') return files;
		return [...files].sort(
			(a, b) => a.name.localeCompare(b.name) * (sortDir === 'asc' ? 1 : -1)
		);
	});

	function simulateIndex(id: number) {
		const timer = setTimeout(
			() => {
				const file = files.find((item) => item.id === id);
				if (file && file.status === 'processing') file.status = 'indexed';
			},
			2600 + Math.random() * 1800
		);
		timers.push(timer);
	}

	function showError(message: string) {
		error = message;
		clearTimeout(errorTimer);
		errorTimer = setTimeout(() => (error = ''), 4000);
	}

	function formatSize(bytes: number) {
		if (bytes < 1024 * 1024) return `${Math.max(1, Math.round(bytes / 1024))} KB`;
		return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
	}

	function addFiles(list: FileList | null) {
		if (!list) return;

		for (const file of Array.from(list)) {
			const ext = file.name.split('.').pop()?.toLowerCase() as FileType | undefined;

			if (!ext || !ALLOWED.includes(ext)) {
				showError('仅支持 PDF、TXT、MD 或 DOCX 文件');
				continue;
			}

			if (file.size > MAX_SIZE) {
				showError(`${file.name} 超过 10 MB 限制`);
				continue;
			}

			const id = nextId++;
			files.unshift({
				id,
				name: file.name,
				size: formatSize(file.size),
				added: '刚刚',
				type: ext,
				status: 'processing'
			});
			simulateIndex(id);
		}
	}

	function handleInput(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		addFiles(input.files);
		input.value = '';
	}

	function handleDrop(event: DragEvent) {
		event.preventDefault();
		dragOver = false;
		addFiles(event.dataTransfer?.files ?? null);
	}

	function removeFile(id: number) {
		files = files.filter((file) => file.id !== id);
	}

	function cycleSort() {
		sortDir = sortDir === 'asc' ? 'desc' : 'asc';
	}

	onMount(() => {
		for (const file of files) {
			if (file.status === 'processing') simulateIndex(file.id);
		}

		return () => {
			timers.forEach((timer) => clearTimeout(timer));
			clearTimeout(errorTimer);
		};
	});
</script>

<svelte:head>
	<title>知识库 · Orb Console</title>
</svelte:head>

<div>
	<h1 class="text-[21px] font-semibold tracking-[-0.02em] text-zinc-900">知识库</h1>
	<p class="mt-1 text-[12.5px] text-zinc-400">上传文档，Orb 会自动完成切分与索引。</p>
</div>

<input
	bind:this={inputEl}
	id="knowledge-upload"
	type="file"
	class="sr-only"
	accept=".pdf,.txt,.md,.docx"
	multiple
	onchange={handleInput}
/>
<label
	for="knowledge-upload"
	ondragover={(event) => {
		event.preventDefault();
		dragOver = true;
	}}
	ondragleave={() => (dragOver = false)}
	ondrop={handleDrop}
	class={`mt-5 block cursor-pointer rounded-2xl border border-dashed px-6 py-10 text-center transition ${
		dragOver
			? 'border-zinc-500 bg-zinc-50'
			: 'border-zinc-300 bg-white hover:border-zinc-400 hover:bg-zinc-50/70'
	}`}
>
	<span class="mx-auto grid h-11 w-11 place-items-center rounded-full bg-zinc-100 text-zinc-500">
		<AdminIcon name="upload" class="h-5 w-5" />
	</span>
	<span class="mt-3.5 block text-[13.5px] font-medium text-zinc-700">
		点击上传，或将文件拖拽到此处
	</span>
	<span class="mt-1 block text-[12px] text-zinc-400">支持 PDF、TXT、MD 或 DOCX（最大 10MB）</span>
	{#if error}
		<span class="mt-2 block text-[12px] text-red-500" role="alert">{error}</span>
	{/if}
</label>

<div class="mt-9 flex flex-wrap items-center justify-between gap-3">
	<nav class="flex items-center gap-1.5 text-[12.5px] text-zinc-400" aria-label="面包屑">
		<span>首页</span>
		<AdminIcon name="chevron-right" class="h-3 w-3 text-zinc-300" />
		<span class="font-medium text-zinc-700">知识库</span>
	</nav>

	<div class="flex items-center gap-2">
		<button
			type="button"
			onclick={() => (view = view === 'list' ? 'grid' : 'list')}
			class="grid h-9 w-9 place-items-center rounded-[10px] border border-zinc-200 bg-white text-zinc-500 shadow-sm transition hover:border-zinc-300 hover:text-zinc-800"
			aria-label={view === 'list' ? '切换为网格视图' : '切换为列表视图'}
		>
			<AdminIcon name={view === 'list' ? 'layout-grid' : 'list-bullet'} class="h-4 w-4" />
		</button>
		<button
			type="button"
			onclick={() => inputEl?.click()}
			class="inline-flex h-9 items-center gap-1.5 rounded-[10px] bg-zinc-700 px-4 text-[13px] font-medium text-white shadow-sm transition hover:bg-zinc-800 active:scale-[0.99]"
		>
			<AdminIcon name="plus" class="h-3.5 w-3.5" strokeWidth={2} />
			新建
		</button>
	</div>
</div>

<div
	class="mt-4 hidden grid-cols-[minmax(0,1fr)_96px_104px_76px_32px] items-center gap-3 px-4 pb-1 sm:grid"
>
	<button
		type="button"
		onclick={cycleSort}
		class="flex items-center gap-1 text-[10.5px] font-semibold uppercase tracking-[0.1em] text-zinc-400 transition hover:text-zinc-600"
	>
		File name
		<AdminIcon
			name="chevron-down"
			class={`h-2.5 w-2.5 transition-transform ${sortDir === 'desc' ? 'rotate-180' : ''}`}
			strokeWidth={2}
		/>
	</button>
	<span class="text-[10.5px] font-semibold uppercase tracking-[0.1em] text-zinc-400">Status</span>
	<span class="text-[10.5px] font-semibold uppercase tracking-[0.1em] text-zinc-400">Added</span>
	<span class="text-right text-[10.5px] font-semibold uppercase tracking-[0.1em] text-zinc-400"
		>Size</span
	>
	<span></span>
</div>

{#if files.length === 0}
	<div
		class="mt-2 rounded-xl border border-dashed border-zinc-300 bg-white/60 py-12 text-center text-[12.5px] text-zinc-400"
	>
		暂无文件，上传后会自动出现在这里。
	</div>
{:else if view === 'list'}
	<div class="mt-2 space-y-2">
		{#each sortedFiles as file (file.id)}
			<div
				class="group grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3 rounded-xl border border-zinc-200/90 bg-white px-3.5 py-3 transition hover:border-zinc-300 hover:shadow-[0_2px_12px_rgba(24,24,27,0.05)] sm:grid-cols-[minmax(0,1fr)_96px_104px_76px_32px] sm:px-4"
			>
				<div class="flex min-w-0 items-center gap-3">
					<span class="grid h-9 w-9 shrink-0 place-items-center rounded-lg {typeStyles[file.type]}">
						<AdminIcon name="document" class="h-[18px] w-[18px]" />
					</span>
					<div class="min-w-0">
						<p class="truncate text-[13px] font-medium text-zinc-800">{file.name}</p>
						<p class="mt-0.5 text-[11px] text-zinc-400 sm:hidden">
							{file.added} · {file.size}
						</p>
					</div>
				</div>

				<div class="flex items-center justify-self-end gap-1 sm:contents">
					<span
						class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[10.5px] font-medium {statusMeta[
							file.status
						].badge}"
					>
						{#if file.status === 'processing'}
							<span class="h-1 w-1 animate-pulse rounded-full bg-amber-500"></span>
						{/if}
						{statusMeta[file.status].label}
					</span>
					<button
						type="button"
						onclick={() => removeFile(file.id)}
						class="grid h-7 w-7 place-items-center rounded-md text-zinc-300 transition hover:bg-red-50 hover:text-red-500 sm:hidden"
						aria-label={`删除 ${file.name}`}
					>
						<AdminIcon name="trash" class="h-4 w-4" />
					</button>
				</div>

				<span class="hidden text-[12px] text-zinc-400 sm:block">{file.added}</span>
				<span class="hidden text-right text-[12px] tabular-nums text-zinc-400 sm:block">
					{file.size}
				</span>
				<button
					type="button"
					onclick={() => removeFile(file.id)}
					class="hidden h-7 w-7 place-items-center justify-self-end rounded-md text-zinc-300 opacity-0 transition hover:bg-red-50 hover:text-red-500 focus-visible:opacity-100 group-hover:opacity-100 sm:grid"
					aria-label={`删除 ${file.name}`}
				>
					<AdminIcon name="trash" class="h-4 w-4" />
				</button>
			</div>
		{/each}
	</div>
{:else}
	<div class="mt-3 grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
		{#each sortedFiles as file (file.id)}
			<div
				class="group rounded-xl border border-zinc-200/90 bg-white p-4 transition hover:border-zinc-300 hover:shadow-[0_2px_12px_rgba(24,24,27,0.05)]"
			>
				<div class="flex items-start justify-between">
					<span class="grid h-9 w-9 place-items-center rounded-lg {typeStyles[file.type]}">
						<AdminIcon name="document" class="h-[18px] w-[18px]" />
					</span>
					<button
						type="button"
						onclick={() => removeFile(file.id)}
						class="grid h-7 w-7 place-items-center rounded-md text-zinc-300 opacity-0 transition hover:bg-red-50 hover:text-red-500 focus-visible:opacity-100 group-hover:opacity-100"
						aria-label={`删除 ${file.name}`}
					>
						<AdminIcon name="trash" class="h-4 w-4" />
					</button>
				</div>
				<p class="mt-3 truncate text-[13px] font-medium text-zinc-800">{file.name}</p>
				<div class="mt-2 flex items-center justify-between">
					<span
						class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[10.5px] font-medium {statusMeta[
							file.status
						].badge}"
					>
						{#if file.status === 'processing'}
							<span class="h-1 w-1 animate-pulse rounded-full bg-amber-500"></span>
						{/if}
						{statusMeta[file.status].label}
					</span>
					<span class="text-[11px] tabular-nums text-zinc-400">{file.size}</span>
				</div>
				<p class="mt-2 text-[11px] text-zinc-400">添加于 {file.added}</p>
			</div>
		{/each}
	</div>
{/if}
