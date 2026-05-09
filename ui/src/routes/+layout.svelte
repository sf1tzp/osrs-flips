<script lang="ts">
	import './layout.css';
	import favicon from '$lib/assets/favicon.svg';

	import { ModeWatcher } from 'mode-watcher';
	import NavBar from '$lib/components/nav-bar.svelte';

	let { children } = $props();
</script>

<svelte:head><link rel="icon" href={favicon} /></svelte:head>
<ModeWatcher />
<div class="flex min-h-screen flex-col">
	<NavBar />
	<div class="flex-1">
		{@render children()}
	</div>
</div>
