<script lang="ts">
	import { page } from '$app/state';
	import { Button } from '$lib/components/ui/button';

	const links = [
		{ href: '/', label: 'Prices' },
		{ href: '/signals', label: 'Signals' },
		{ href: '/portfolio', label: 'Portfolio' },
		{ href: '/faq', label: 'FAQ' }
	];
</script>

<nav class="border-b bg-muted/50">
	<div class="mx-auto flex max-w-7xl items-center gap-1 px-4 py-2 sm:px-6">
		{#each links as link (link.href)}
			{@const active =
				link.href === '/' ? page.url.pathname === '/' : page.url.pathname.startsWith(link.href)}
			<Button variant={active ? 'secondary' : 'ghost'} size="sm" href={link.href}>
				{link.label}
			</Button>
		{/each}
	</div>
</nav>
