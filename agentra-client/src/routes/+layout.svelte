<script lang="ts">
	import '../app.css';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import { LayoutDashboard, FolderKanban, Image, Activity, Database, Settings2, LogOut } from 'lucide-svelte';
	import { auth, init, setUser, clear } from '$lib/auth.svelte';
	import { activity, start, stop } from '$lib/activity.svelte';
	import BackendActivityButton from '$lib/components/BackendActivityButton.svelte';

	let { children } = $props();
	let ready = $state(false);

	const PUBLIC = ['/login', '/setup'];
	const isPublicRoute = $derived(PUBLIC.some((p) => $page.url.pathname.startsWith(p)));

	onMount(() => {
		let disposed = false;
		(async () => {
			init();

			if (PUBLIC.some((p) => $page.url.pathname.startsWith(p))) {
				ready = true;
				return;
			}

			if (!auth.token) {
				goto('/login');
				return;
			}

			const res = await api.get('/me');
			if (!res.ok) {
				clear();
				goto('/login');
				return;
			}
			setUser(await res.json());
			if (disposed) return;
			ready = true;
			void start();
		})();

		return () => {
			disposed = true;
			stop();
		};
	});

	async function handleLogout() {
		stop();
		await api.post('/logout');
		clear();
		goto('/login');
	}
</script>

{#if ready}
	{#if auth.user && !isPublicRoute}
		<div class="app-shell overflow-hidden h-screen">
			<div class="flex h-full flex-col md:flex-row">
				<aside class="bg-slate-950 text-slate-100 md:w-64 w-full flex-shrink-0 overflow-y-auto border-r border-white/5">
					<div class="px-6 py-8 flex items-center gap-3">
						<div class="brand-chip brand-mark">agentra</div>
					</div>
					<nav class="px-3 pb-6 flex flex-wrap gap-1 md:flex-col md:gap-1">
						<a href="/dashboard" class="nav-chip {$page.url.pathname.startsWith('/dashboard') ? 'nav-chip-active' : ''}">
							<LayoutDashboard size={18} />
							<span>Dashboard</span>
						</a>
						<a href="/projects" class="nav-chip {$page.url.pathname.startsWith('/projects') ? 'nav-chip-active' : ''}">
							<FolderKanban size={18} />
							<span>Projects</span>
						</a>
						<a href="/media" class="nav-chip {$page.url.pathname.startsWith('/media') ? 'nav-chip-active' : ''}">
							<Image size={18} />
							<span>Media</span>
						</a>

						{#if auth.user.role === 'admin'}
							<div class="px-4 mt-6 mb-2 text-[10px] uppercase tracking-[0.15em] text-slate-500 font-bold">Administration</div>
							<a href="/admin/pipeline" class="nav-chip {$page.url.pathname.startsWith('/admin/pipeline') ? 'nav-chip-active' : ''}">
								<Activity size={18} />
								<span>Analyzers / Editors</span>
							</a>
							<a href="/admin/storage" class="nav-chip {$page.url.pathname.startsWith('/admin/storage') ? 'nav-chip-active' : ''}">
								<Database size={18} />
								<span>Storage</span>
							</a>
							<a href="/admin/misc" class="nav-chip {$page.url.pathname.startsWith('/admin/misc') ? 'nav-chip-active' : ''}">
								<Settings2 size={18} />
								<span>Misc</span>
							</a>
						{/if}
					</nav>

					<div class="mt-auto px-3 pb-4 md:fixed md:bottom-0 md:w-64">
						<button type="button" class="nav-chip w-full text-left" onclick={handleLogout}>
							<LogOut size={18} />
							<span>Sign out</span>
						</button>
					</div>
				</aside>

				<div class="flex-1 flex flex-col min-w-0 overflow-hidden">
					{#if !$page.url.pathname.startsWith('/media')}
						<header class="px-6 md:px-10 py-4 border-b border-slate-200 bg-white/70 backdrop-blur flex-shrink-0">
							<div class="flex flex-wrap items-center gap-3">
								<div>
									<div class="text-[10px] text-slate-400 uppercase tracking-widest font-bold">Signed in</div>
									<div class="font-semibold text-slate-900 leading-tight">{auth.user.email}</div>
								</div>
								<div class="ml-auto flex items-center gap-4">
									<div class="hidden md:block">
										<BackendActivityButton />
									</div>
									<div class="text-right hidden sm:block">
										<div class="text-[10px] text-slate-400 uppercase tracking-widest font-bold">Access Level</div>
										<div class="text-xs font-semibold text-slate-700 capitalize">{auth.user.role}</div>
										{#if activity.updatedAt}
											<div class="text-[10px] text-slate-400 mt-0.5">{new Date(activity.updatedAt).toLocaleTimeString()}</div>
										{/if}
									</div>
								</div>
							</div>
						</header>
					{/if}
					<main class="flex-1 overflow-y-auto {$page.url.pathname.startsWith('/media') ? '' : 'px-6 md:px-10 py-8'} bg-slate-50/30">
						{@render children()}
					</main>
				</div>
			</div>
		</div>
	{:else}
		<main class="min-h-screen">
			{@render children()}
		</main>
	{/if}
{/if}
