<script lang="ts">
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();
</script>

<div class="min-h-dvh bg-[#f6f6f7] font-sans text-zinc-900">
	<header class="border-b border-zinc-200/80 bg-white">
		<div class="mx-auto flex h-14 w-full max-w-[880px] items-center gap-2.5 px-5 sm:px-8">
			<span
				class="h-6 w-6 rounded-full bg-[radial-gradient(circle_at_32%_28%,#d1fae5_0%,#6ee7b7_22%,#10b981_52%,#047857_78%,#064e3b_100%)] shadow-[inset_0_-2px_4px_rgba(6,78,59,0.35)]"
			></span>
			<span class="text-[13.5px] font-semibold tracking-tight text-zinc-900">Orb</span>
			<span
				class="rounded-full border border-zinc-200 bg-zinc-50 px-2 py-0.5 text-[10px] font-medium text-zinc-500"
			>
				管理端
			</span>
			<a
				href="/"
				rel="external"
				class="ml-auto text-[12px] text-zinc-400 transition hover:text-zinc-700"
			>
				用户端
			</a>
		</div>
	</header>

	<main class="mx-auto w-full max-w-[880px] px-5 py-8 sm:px-8 sm:py-10">
		{@render children()}
	</main>
</div>

<style>
	:global(html),
	:global(body) {
		background: #f6f6f7;
		color-scheme: light;
	}
</style>
