<script lang="ts">
	import { api } from '$lib/api';
	import { auth } from '$lib/auth.svelte';
	import ConfirmModal from '$lib/components/ConfirmModal.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import ProviderModal from '$lib/components/ProviderModal.svelte';
	import type { ProviderModalMode } from '$lib/model-admin';
	import { getAnalyzerModelHint, getProviderRule } from '$lib/model-setup';
	import { onMount } from 'svelte';

	type AnalyzerType = 'transcription' | 'vision_tags' | 'embedding';
	type TestRunType = 'transcription' | 'vision_tags' | 'embedding';

	type TestTerminalEvent = {
		event: string;
		message: string;
		timestamp: string;
		payload?: Record<string, any>;
	};

	type TestSummary = {
		successes: number;
		failures: number;
		warnings: number;
	};

	type TestMeta = {
		label: string;
		description: string;
	};

	type EditorAgentConfig = {
		provider_id: string;
		model_name: string;
		base_prompt: string;
		max_duration_sec: number;
		is_autonomous_enabled: boolean;
	};

	let providers = $state<any[]>([]);
	let analyzers = $state<any[]>([]);
	let providerTypes = $state<any[]>([]);
	let pageError = $state('');

	let editorConfig = $state<EditorAgentConfig>({
		provider_id: '',
		model_name: '',
		base_prompt: '',
		max_duration_sec: 300,
		is_autonomous_enabled: false
	});
	let editorSaving = $state(false);
	let editorSaveMessage = $state('');
	let editorError = $state('');
	let editorTestOpen = $state(false);
	let editorTestRunning = $state(false);
	let editorTestEvents = $state<TestTerminalEvent[]>([]);
	let editorTestSummary = $state<TestSummary | null>(null);
	let editorTestError = $state('');
	let editorTestTerminalEl = $state<HTMLDivElement>();
	let editorTestAbortController: AbortController | null = null;

	onMount(async () => {
		const [pRes, aRes, tRes, eRes] = await Promise.all([
			api.get('/admin/providers'),
			api.get('/admin/analyzers'),
			api.get('/admin/providers/types'),
			api.get('/editor-config')
		]);
		if (pRes.ok) providers = await pRes.json();
		if (aRes.ok) analyzers = await aRes.json();
		if (tRes.ok) providerTypes = await tRes.json();
		if (eRes.ok) {
			const cfg = await eRes.json();
			editorConfig = {
				provider_id: cfg.provider_id ?? '',
				model_name: cfg.model_name ?? '',
				base_prompt: cfg.base_prompt ?? '',
				max_duration_sec: Number(cfg.max_duration_sec ?? 300),
				is_autonomous_enabled: Boolean(cfg.is_autonomous_enabled)
			};
		}
	});

	const providerTypesMeta = $derived(
		Object.fromEntries(providerTypes.map((t: any) => [t.type, t]))
	);

	const enabledAnalyzers = $derived(analyzers.filter((a: any) => a.is_enabled));
	const hasEmbedding = $derived(enabledAnalyzers.some((a: any) => a.analyzer_type === 'embedding'));
	const hasSignal = $derived(
		enabledAnalyzers.some((a: any) => a.analyzer_type === 'transcription' || a.analyzer_type === 'vision_tags')
	);
	const pipelineReady = $derived(hasEmbedding && hasSignal);

	const analyzerMeta: Record<string, { label: string; hint: string; chip: string }> = {
		transcription: {
			label: 'Transcription',
			hint: 'Extract spoken text from audio.',
			chip: 'bg-sky-100 text-sky-800 border-sky-200'
		},
		vision_tags: {
			label: 'Vision Tags',
			hint: 'Detect objects and scene tags.',
			chip: 'bg-amber-100 text-amber-800 border-amber-200'
		},
		embedding: {
			label: 'Embedding',
			hint: 'Create vectors for search and retrieval.',
			chip: 'bg-emerald-100 text-emerald-800 border-emerald-200'
		}
	};

	let providerModalOpen = $state(false);
	let providerMode = $state<ProviderModalMode>('create');
	let editingProvider = $state<any>(null);

	let analyzerModalOpen = $state(false);
	let analyzerSubmitting = $state(false);
	let analyzerMode = $state<'create' | 'edit'>('create');
	let analyzerId = $state('');
	let analyzerName = $state('');
	let analyzerNameAutofill = $state(true);
	let analyzerType = $state<AnalyzerType>('transcription');
	let analyzerProviderId = $state('');
	let analyzerModelName = $state('');
	let analyzerConfigJson = $state('{}');
	let analyzerIsEnabled = $state(true);
	let analyzerError = $state('');

	let testModalOpen = $state(false);
	let testRunning = $state(false);
	let testAnalyzer: any = $state(null);
	let testRunType = $state<TestRunType>('transcription');
	let testTerminalEvents = $state<TestTerminalEvent[]>([]);
	let testSummary = $state<TestSummary | null>(null);
	let testError = $state('');
	let testTerminalEl = $state<HTMLDivElement>();
	let testAbortController: AbortController | null = null;

	const testMetaByType: Record<TestRunType, TestMeta> = {
		transcription: {
			label: 'Transcription',
			description: 'Runs real transcription analyzer logic against backend audio samples and streams progress events.'
		},
		vision_tags: {
			label: 'Vision Tags',
			description: 'Runs real vision tag analyzer logic against backend image samples and streams progress events.'
		},
		embedding: {
			label: 'Embedding',
			description: 'Runs real embedding analyzer logic against backend text samples and streams progress events.'
		}
	};

	let confirmDeleteOpen = $state(false);
	let confirmDeleteType = $state<'provider' | 'analyzer'>('provider');
	let confirmDeleteId = $state('');
	let confirmDeleteName = $state('');
	let confirmDeleteSubmitting = $state(false);

	const selectedAnalyzerProvider = $derived(providers.find((p: any) => p.id === analyzerProviderId));
	const editorProviderOptions = $derived(
		providers.filter((p: any) => p.provider_type !== 'deepgram')
	);
	const selectedAnalyzerProviderType = $derived(selectedAnalyzerProvider?.provider_type ?? 'other');
	const selectedAnalyzerRules = $derived(getProviderRule(selectedAnalyzerProviderType));
	const analyzerTypeLockedByProvider = $derived(Boolean(selectedAnalyzerRules.forcedAnalyzerType));
	const analyzerModelRequired = $derived(selectedAnalyzerRules.modelMode === 'required');
	const analyzerModelDisabled = $derived(selectedAnalyzerRules.modelMode === 'disabled');
	const analyzerModelHint = $derived(getAnalyzerModelHint(selectedAnalyzerProviderType));

	$effect(() => {
		testTerminalEvents.length;
		if (!testTerminalEl) return;
		testTerminalEl.scrollTop = testTerminalEl.scrollHeight;
	});

	$effect(() => {
		editorTestEvents.length;
		if (!editorTestTerminalEl) return;
		editorTestTerminalEl.scrollTop = editorTestTerminalEl.scrollHeight;
	});

	$effect(() => {
		if (!analyzerModalOpen) return;
		if (selectedAnalyzerRules.forcedAnalyzerType) {
			analyzerType = selectedAnalyzerRules.forcedAnalyzerType;
		}
		if (analyzerModelDisabled) {
			analyzerModelName = '';
		}
		if (analyzerNameAutofill) {
			analyzerName = suggestedAnalyzerName();
		}
	});

	$effect(() => {
		if (editorConfig.provider_id) return;
		const first = editorProviderOptions[0];
		if (!first) return;
		editorConfig = { ...editorConfig, provider_id: first.id };
	});

	function providerTypeLabel(t: string) {
		return providerTypesMeta[t]?.label ?? t;
	}

	function firstAnalyzerProviderId() {
		return providers.find((p: any) => p.is_active)?.id ?? providers[0]?.id ?? '';
	}

	function openCreateProvider() {
		providerMode = 'create';
		editingProvider = null;
		providerModalOpen = true;
	}

	function openEditProvider(p: any) {
		providerMode = 'edit';
		editingProvider = p;
		providerModalOpen = true;
	}

	function onProviderSaved(saved: any) {
		const exists = providers.some((p: any) => p.id === saved.id);
		if (exists) {
			providers = providers.map((p: any) => (p.id === saved.id ? saved : p));
			return;
		}
		providers = [...providers, saved];
	}

	function openCreateAnalyzer() {
		analyzerMode = 'create';
		analyzerId = '';
		analyzerProviderId = firstAnalyzerProviderId();
		analyzerType = 'transcription';
		analyzerModelName = '';
		analyzerConfigJson = '{}';
		analyzerIsEnabled = true;
		analyzerNameAutofill = true;
		analyzerName = suggestedAnalyzerName();
		analyzerError = '';
		analyzerModalOpen = true;
	}

	function openEditAnalyzer(a: any) {
		analyzerMode = 'edit';
		analyzerId = a.id;
		analyzerProviderId = a.provider_id;
		analyzerType = a.analyzer_type;
		analyzerModelName = a.model_name;
		analyzerConfigJson = cfg(a.config_json);
		analyzerIsEnabled = a.is_enabled;
		analyzerName = a.name;
		analyzerNameAutofill = false;
		analyzerError = '';
		analyzerModalOpen = true;
	}

	function onAnalyzerProviderChange() {
		if (selectedAnalyzerRules.forcedAnalyzerType) {
			analyzerType = selectedAnalyzerRules.forcedAnalyzerType;
		}
		if (analyzerNameAutofill) {
			analyzerName = suggestedAnalyzerName();
		}
	}

	function onAnalyzerTypeChange() {
		if (analyzerNameAutofill) {
			analyzerName = suggestedAnalyzerName();
		}
	}

	function onAnalyzerNameInput() {
		analyzerNameAutofill = false;
	}

	function applyAnalyzerNameAutofill() {
		analyzerNameAutofill = true;
		analyzerName = suggestedAnalyzerName();
	}

	function suggestedAnalyzerName() {
		const pName = selectedAnalyzerProvider?.name ? String(selectedAnalyzerProvider.name).trim() : 'Unnamed Provider';
		return `${pName} ${analyzerLabel(analyzerType)} Analyzer`;
	}

	function openDeleteProviderConfirm(p: any) {
		confirmDeleteType = 'provider';
		confirmDeleteId = p.id;
		confirmDeleteName = p.name;
		confirmDeleteOpen = true;
	}

	function openDeleteAnalyzerConfirm(a: any) {
		confirmDeleteType = 'analyzer';
		confirmDeleteId = a.id;
		confirmDeleteName = a.name;
		confirmDeleteOpen = true;
	}

	function closeDeleteConfirm() {
		if (!confirmDeleteSubmitting) confirmDeleteOpen = false;
	}

	function cfg(v: any) {
		if (!v) return '{}';
		if (typeof v === 'string') return v;
		return JSON.stringify(v, null, 2);
	}

	function analyzerLabel(t: string) {
		return analyzerMeta[t]?.label ?? t;
	}

	function analyzerHint(t: string) {
		return analyzerMeta[t]?.hint ?? '';
	}

	function analyzerChip(t: string) {
		return analyzerMeta[t]?.chip ?? 'bg-slate-100 text-slate-800 border-slate-200';
	}

	async function submitAnalyzer() {
		analyzerSubmitting = true;
		analyzerError = '';
		const body = {
			name: analyzerName,
			analyzer_type: analyzerType,
			provider_id: analyzerProviderId,
			model_name: analyzerModelName,
			config_json: analyzerConfigJson,
			is_enabled: analyzerIsEnabled
		};
		const res =
			analyzerMode === 'create'
				? await api.post('/admin/analyzers', body)
				: await api.put(`/admin/analyzers/${analyzerId}`, body);
		analyzerSubmitting = false;
		if (!res.ok) {
			const data = await res.json().catch(() => ({}));
			analyzerError = data.error ?? 'Failed to save analyzer';
			return;
		}
		const saved = await res.json();
		if (analyzerMode === 'create') {
			analyzers = [...analyzers, saved];
		} else {
			analyzers = analyzers.map((a) => (a.id === analyzerId ? saved : a));
		}
		analyzerModalOpen = false;
	}

	async function toggleAnalyzer(a: any) {
		const res = await api.put(`/admin/analyzers/${a.id}`, {
			name: a.name,
			analyzer_type: a.analyzer_type,
			provider_id: a.provider_id,
			model_name: a.model_name,
			config_json: cfg(a.config_json),
			is_enabled: !a.is_enabled
		});
		if (res.ok) {
			const saved = await res.json();
			analyzers = analyzers.map((x) => (x.id === a.id ? saved : x));
		}
	}

	async function confirmDelete() {
		confirmDeleteSubmitting = true;
		if (confirmDeleteType === 'provider') {
			const res = await api.del(`/admin/providers/${confirmDeleteId}`);
			if (res.ok) {
				providers = providers.filter((p) => p.id !== confirmDeleteId);
			} else {
				const data = await res.json().catch(() => ({}));
				pageError = data.error ?? 'Failed to delete provider';
			}
		} else {
			const res = await api.del(`/admin/analyzers/${confirmDeleteId}`);
			if (res.ok) {
				analyzers = analyzers.filter((a) => a.id !== confirmDeleteId);
			} else {
				const data = await res.json().catch(() => ({}));
				pageError = data.error ?? 'Failed to delete analyzer';
			}
		}
		confirmDeleteSubmitting = false;
		confirmDeleteOpen = false;
	}

	async function saveEditorAgent() {
		editorSaving = true;
		editorError = '';
		editorSaveMessage = '';
		const res = await api.put('/editor-config', {
			provider_id: editorConfig.provider_id,
			model_name: editorConfig.model_name,
			base_prompt: editorConfig.base_prompt,
			max_duration_sec: Number(editorConfig.max_duration_sec),
			is_autonomous_enabled: Boolean(editorConfig.is_autonomous_enabled)
		});
		editorSaving = false;
		if (!res.ok) {
			const body = await res.json().catch(() => ({}));
			editorError = body.error ?? 'Failed to save editor agent';
			return;
		}
		const saved = await res.json();
		editorConfig = {
			provider_id: saved.provider_id ?? editorConfig.provider_id,
			model_name: saved.model_name ?? '',
			base_prompt: saved.base_prompt ?? '',
			max_duration_sec: Number(saved.max_duration_sec ?? 300),
			is_autonomous_enabled: Boolean(saved.is_autonomous_enabled)
		};
		editorSaveMessage = 'Editor agent saved.';
	}

	function openEditorTestModal() {
		editorTestOpen = true;
		editorTestEvents = [];
		editorTestSummary = null;
		editorTestError = '';
	}

	function closeEditorTestModal() {
		stopEditorTest();
		editorTestOpen = false;
	}

	function stopEditorTest() {
		if (editorTestAbortController) {
			editorTestAbortController.abort();
			editorTestAbortController = null;
		}
		editorTestRunning = false;
	}

	function appendEditorTestEvent(event: string, data: any) {
		const payload = data && typeof data === 'object' ? data : {};
		const message = typeof payload.message === 'string' ? payload.message : JSON.stringify(payload);
		const timestamp = typeof payload.timestamp_utc === 'string' ? payload.timestamp_utc : new Date().toISOString();
		const next: TestTerminalEvent = {
			event,
			message,
			timestamp,
			payload: payload.payload && typeof payload.payload === 'object' ? payload.payload : undefined
		};
		const sizeLimit = 800;
		const trimmed = editorTestEvents.length >= sizeLimit ? editorTestEvents.slice(editorTestEvents.length - sizeLimit + 1) : editorTestEvents;
		editorTestEvents = [...trimmed, next];
	}

	function parseEditorSSEBlock(block: string) {
		let eventName = 'message';
		const dataLines: string[] = [];
		for (const line of block.split('\n')) {
			if (!line || line.startsWith(':')) continue;
			if (line.startsWith('event:')) {
				eventName = line.slice(6).trim() || 'message';
				continue;
			}
			if (line.startsWith('data:')) {
				dataLines.push(line.slice(5).trimStart());
			}
		}
		if (eventName === 'heartbeat') return;
		const payloadRaw = dataLines.join('\n');
		if (!payloadRaw) return;
		let payload: any = payloadRaw;
		try {
			payload = JSON.parse(payloadRaw);
		} catch {}
		appendEditorTestEvent(eventName, payload);
		if ((eventName === 'run.completed' || eventName === 'run.failed') && payload?.payload) {
			editorTestSummary = {
				successes: Number(payload.payload.successes ?? 0),
				failures: Number(payload.payload.failures ?? 0),
				warnings: Number(payload.payload.warnings ?? 0)
			};
		}
	}

	async function startEditorTest() {
		stopEditorTest();
		editorTestRunning = true;
		editorTestError = '';
		editorTestSummary = null;
		editorTestEvents = [];
		const controller = new AbortController();
		editorTestAbortController = controller;

		try {
			const res = await fetch(`${api.BASE}/admin/editor-agent/test/stream`, {
				method: 'GET',
				headers: { Accept: 'text/event-stream', Authorization: `Bearer ${auth.token ?? ''}` },
				signal: controller.signal
			});
			if (!res.ok || !res.body) {
				let message = `Editor test failed (${res.status})`;
				try {
					const body = await res.json();
					if (body?.error) message = body.error;
				} catch {}
				throw new Error(message);
			}
			const reader = res.body.getReader();
			const decoder = new TextDecoder();
			let buffer = '';
			while (true) {
				const { done, value } = await reader.read();
				if (done) break;
				buffer += decoder.decode(value, { stream: true }).replace(/\r/g, '');
				let splitAt = buffer.indexOf('\n\n');
				while (splitAt >= 0) {
					const block = buffer.slice(0, splitAt).trim();
					buffer = buffer.slice(splitAt + 2);
					if (block) parseEditorSSEBlock(block);
					splitAt = buffer.indexOf('\n\n');
				}
			}
			const tail = decoder.decode().replace(/\r/g, '');
			if (tail) buffer += tail;
			if (buffer.trim()) parseEditorSSEBlock(buffer.trim());
		} catch (err: any) {
			if (controller.signal.aborted) {
				appendEditorTestEvent('run.cancelled', { message: 'Editor test stopped by user.', timestamp_utc: new Date().toISOString() });
			} else {
				editorTestError = err?.message ?? 'Failed to run editor test';
				appendEditorTestEvent('run.error', { message: editorTestError, timestamp_utc: new Date().toISOString() });
			}
		} finally {
			if (editorTestAbortController === controller) editorTestAbortController = null;
			editorTestRunning = false;
		}
	}

	function openAnalyzerTest(analyzer: any) {
		testAnalyzer = analyzer;
		if (analyzer?.analyzer_type === 'vision_tags') {
			testRunType = 'vision_tags';
		} else if (analyzer?.analyzer_type === 'embedding') {
			testRunType = 'embedding';
		} else {
			testRunType = 'transcription';
		}
		testModalOpen = true;
		testTerminalEvents = [];
		testSummary = null;
		testError = '';
	}

	function closeAnalyzerTestModal() {
		stopAnalyzerTest();
		testModalOpen = false;
	}

	function stopAnalyzerTest() {
		if (testAbortController) {
			testAbortController.abort();
			testAbortController = null;
		}
		testRunning = false;
	}

	function appendTerminalEvent(event: string, data: any) {
		const payload = data && typeof data === 'object' ? data : {};
		const message = typeof payload.message === 'string' ? payload.message : JSON.stringify(payload);
		const timestamp = typeof payload.timestamp_utc === 'string' ? payload.timestamp_utc : new Date().toISOString();
		const next: TestTerminalEvent = {
			event,
			message,
			timestamp,
			payload: payload.payload && typeof payload.payload === 'object' ? payload.payload : undefined
		};
		const sizeLimit = 800;
		const trimmed = testTerminalEvents.length >= sizeLimit ? testTerminalEvents.slice(testTerminalEvents.length - sizeLimit + 1) : testTerminalEvents;
		testTerminalEvents = [...trimmed, next];
	}

	function parseSSEBlock(block: string) {
		let eventName = 'message';
		const dataLines: string[] = [];
		for (const line of block.split('\n')) {
			if (!line || line.startsWith(':')) continue;
			if (line.startsWith('event:')) { eventName = line.slice(6).trim() || 'message'; continue; }
			if (line.startsWith('data:')) { dataLines.push(line.slice(5).trimStart()); }
		}
		if (eventName === 'heartbeat') return;
		const payloadRaw = dataLines.join('\n');
		if (!payloadRaw) return;
		let payload: any = payloadRaw;
		try { payload = JSON.parse(payloadRaw); } catch { }
		appendTerminalEvent(eventName, payload);
		if ((eventName === 'run.completed' || eventName === 'run.failed') && payload?.payload) {
			testSummary = {
				successes: Number(payload.payload.successes ?? 0),
				failures: Number(payload.payload.failures ?? 0),
				warnings: Number(payload.payload.warnings ?? 0)
			};
		}
	}

	async function startAnalyzerTest() {
		if (!testAnalyzer?.id) return;
		stopAnalyzerTest();
		testRunning = true;
		testError = '';
		testSummary = null;
		testTerminalEvents = [];
		const controller = new AbortController();
		testAbortController = controller;
		const url = `${api.BASE}/admin/analyzers/${testAnalyzer.id}/test/stream`;
		try {
			const res = await fetch(url, {
				method: 'GET',
				headers: { Accept: 'text/event-stream', Authorization: `Bearer ${auth.token ?? ''}` },
				signal: controller.signal
			});
			if (!res.ok || !res.body) {
				let message = `Test stream failed (${res.status})`;
				try { const body = await res.json(); if (body?.error) message = body.error; } catch { }
				throw new Error(message);
			}
			const reader = res.body.getReader();
			const decoder = new TextDecoder();
			let buffer = '';
			while (true) {
				const { done, value } = await reader.read();
				if (done) break;
				buffer += decoder.decode(value, { stream: true }).replace(/\r/g, '');
				let splitAt = buffer.indexOf('\n\n');
				while (splitAt >= 0) {
					const block = buffer.slice(0, splitAt).trim();
					buffer = buffer.slice(splitAt + 2);
					if (block) parseSSEBlock(block);
					splitAt = buffer.indexOf('\n\n');
				}
			}
			const tail = decoder.decode().replace(/\r/g, '');
			if (tail) buffer += tail;
			if (buffer.trim()) parseSSEBlock(buffer.trim());
		} catch (err: any) {
			if (controller.signal.aborted) {
				appendTerminalEvent('run.cancelled', { message: 'Test stream stopped by user.', timestamp_utc: new Date().toISOString() });
			} else {
				testError = err?.message ?? 'Failed to run test stream';
				appendTerminalEvent('run.error', { message: testError, timestamp_utc: new Date().toISOString() });
			}
		} finally {
			if (testAbortController === controller) testAbortController = null;
			testRunning = false;
		}
	}

	function prettyTimestamp(ts: string) {
		const parsed = new Date(ts);
		if (Number.isNaN(parsed.valueOf())) return ts;
		return parsed.toLocaleTimeString();
	}
</script>

<svelte:head><title>Analyzers / Editors - Admin - Agentra</title></svelte:head>

<div class="max-w-6xl mx-auto space-y-6">
	<div class="flex items-start justify-between gap-3">
		<div>
			<h1 class="text-3xl font-semibold">Analyzers / Editors</h1>
			<p class="text-sm text-slate-500 mt-1">Providers define endpoints and keys. Analyzers and the editor agent bind provider + model configuration.</p>
		</div>
	</div>

	{#if pageError}
		<div class="p-3 rounded bg-red-50 text-red-700 text-sm">{pageError}</div>
	{/if}

	<div class="surface-card p-6 space-y-4">
		<div class="flex items-center justify-between gap-3">
			<div>
				<h2 class="text-xl font-semibold">Model Providers</h2>
				<p class="text-sm text-slate-500 mt-1">Reusable API endpoint + auth configuration.</p>
			</div>
			<button type="button" class="btn-primary text-sm" onclick={openCreateProvider}>+ Add Provider</button>
		</div>

		<div class="overflow-auto">
			<table class="w-full text-sm">
				<thead>
					<tr class="text-left border-b border-slate-200">
						<th class="py-2">Name</th>
						<th class="py-2">Type</th>
						<th class="py-2">Base URL</th>
						<th class="py-2">API Key</th>
						<th class="py-2">Status</th>
						<th class="py-2">Actions</th>
					</tr>
				</thead>
				<tbody>
					{#each providers as p}
						<tr class="border-b border-slate-100 align-top">
							<td class="py-2 font-medium">{p.name}</td>
							<td class="py-2">
								<span class="inline-flex border rounded-full px-2 py-0.5 text-xs bg-slate-100 border-slate-200 text-slate-700">
									{providerTypeLabel(p.provider_type)}
								</span>
							</td>
							<td class="py-2 font-mono text-xs text-slate-600">{p.base_url}</td>
							<td class="py-2">{p.has_api_key ? 'Configured' : 'Not set'}</td>
							<td class="py-2">
								<span class={`inline-flex border rounded-full px-2 py-0.5 text-xs ${p.is_active ? 'bg-emerald-100 text-emerald-800 border-emerald-200' : 'bg-slate-100 text-slate-700 border-slate-200'}`}>
									{p.is_active ? 'Active' : 'Disabled'}
								</span>
							</td>
							<td class="py-2 flex items-center gap-2">
								<button type="button" class="h-8 w-8 inline-flex items-center justify-center rounded-md border border-slate-200 text-slate-600 hover:bg-slate-100" aria-label="Edit provider" onclick={() => openEditProvider(p)}>
									<svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 20h9" /><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4Z" /></svg>
								</button>
								<button type="button" class="h-8 w-8 inline-flex items-center justify-center rounded-md border border-rose-200 text-rose-600 hover:bg-rose-50" aria-label="Delete provider" onclick={() => openDeleteProviderConfirm(p)}>
									<svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 6h18" /><path d="M8 6V4h8v2" /><path d="M19 6l-1 14H6L5 6" /></svg>
								</button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	</div>

	<div class="surface-card p-6 space-y-4">
		<div class="flex items-center justify-between gap-3">
			<div>
				<h2 class="text-xl font-semibold">Editor Agent</h2>
				<p class="text-sm text-slate-500 mt-1">One editor agent per organization. Configure provider + model and run a live connectivity test.</p>
			</div>
		</div>

		{#if editorError}
			<div class="p-3 rounded bg-red-50 text-red-700 text-sm">{editorError}</div>
		{/if}
		{#if editorSaveMessage}
			<div class="p-3 rounded bg-emerald-50 text-emerald-700 text-sm">{editorSaveMessage}</div>
		{/if}

		<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
			<div>
				<label class="block text-xs font-medium mb-1">Provider</label>
				<select
					class="input-base"
					bind:value={editorConfig.provider_id}
					disabled={editorProviderOptions.length === 0}
				>
					{#if editorProviderOptions.length === 0}
						<option value="">No compatible providers</option>
					{:else}
						{#each editorProviderOptions as p}
							<option value={p.id}>{p.name} ({providerTypeLabel(p.provider_type)})</option>
						{/each}
					{/if}
				</select>
				<div class="mt-1 text-xs text-slate-500">Deepgram providers are excluded for editor agent chat testing.</div>
			</div>
			<div>
				<label class="block text-xs font-medium mb-1">Model Name</label>
				<input type="text" class="input-base" bind:value={editorConfig.model_name} placeholder="gpt-4o-mini" />
			</div>
			<div class="md:col-span-2">
				<label class="block text-xs font-medium mb-1">Base Prompt</label>
				<textarea
					rows="4"
					class="input-base font-mono"
					bind:value={editorConfig.base_prompt}
					placeholder="You are a precise video editor assistant..."
				></textarea>
			</div>
			<div>
				<label class="block text-xs font-medium mb-1">Max Duration (seconds)</label>
				<input type="number" min="1" class="input-base" bind:value={editorConfig.max_duration_sec} />
			</div>
			<div>
				<label class="block text-xs font-medium mb-1">Autonomous Mode</label>
				<label class="text-sm flex items-center gap-2">
					<input type="checkbox" bind:checked={editorConfig.is_autonomous_enabled} />
					Enable autonomous scheduling
				</label>
			</div>
		</div>

		<div class="flex items-center gap-2">
			<button type="button" class="btn-primary text-sm" disabled={editorSaving} onclick={saveEditorAgent}>
				{editorSaving ? 'Saving...' : 'Save Editor Agent'}
			</button>
			<button
				type="button"
				class="btn-secondary text-sm"
				disabled={!editorConfig.provider_id || !editorConfig.model_name}
				onclick={openEditorTestModal}
			>
				Test Editor Agent
			</button>
		</div>
	</div>

	<div class="surface-card p-6 space-y-4">
		<div class="flex items-center justify-between gap-3">
			<div class="min-w-0">
				<div class="flex flex-wrap items-center gap-2">
					<h2 class="text-xl font-semibold">Analyzers</h2>
					<span class={`inline-flex items-center rounded-full border px-2 py-0.5 text-[11px] font-semibold ${pipelineReady ? 'border-emerald-200 bg-emerald-50 text-emerald-700' : 'border-amber-200 bg-amber-50 text-amber-700'}`}>
						{pipelineReady ? 'Ready' : 'Misconfigured'}
					</span>
				</div>
				<p class="text-sm text-slate-500 mt-1">Model + provider + type configuration for the analysis loop.</p>
				{#if !pipelineReady}
					<p class="text-xs text-amber-700 mt-1">Need at least one `embedding` and one `transcription` or `vision_tags` analyzer enabled.</p>
				{/if}
			</div>
			<button type="button" class="btn-primary text-sm disabled:opacity-60 disabled:cursor-not-allowed" onclick={openCreateAnalyzer} disabled={providers.length === 0}>+ Add Analyzer</button>
		</div>

		{#if providers.length === 0}
			<div class="text-sm text-amber-700 bg-amber-50 border border-amber-200 rounded-md p-3">
				Create a provider first before adding analyzers.
			</div>
		{/if}

		<div class="overflow-auto">
			<table class="w-full text-sm">
				<thead>
					<tr class="text-left border-b border-slate-200">
						<th class="py-2">Name</th>
						<th class="py-2">Type</th>
						<th class="py-2">Provider</th>
						<th class="py-2">Model</th>
						<th class="py-2">State</th>
						<th class="py-2">Actions</th>
					</tr>
				</thead>
				<tbody>
					{#each analyzers as a}
						<tr class="border-b border-slate-100 align-top">
							<td class="py-2">
								<div class="font-medium">{a.name}</div>
								<div class="text-xs text-slate-500 mt-0.5">{analyzerHint(a.analyzer_type)}</div>
							</td>
							<td class="py-2">
								<span class={`inline-flex border rounded-full px-2 py-0.5 text-xs ${analyzerChip(a.analyzer_type)}`}>
									{analyzerLabel(a.analyzer_type)}
								</span>
							</td>
							<td class="py-2">{a.provider_name}</td>
							<td class="py-2 font-mono text-xs text-slate-600">{a.model_name}</td>
							<td class="py-2">
								<span class={`inline-flex border rounded-full px-2 py-0.5 text-xs ${a.is_enabled ? 'bg-emerald-100 text-emerald-800 border-emerald-200' : 'bg-slate-100 text-slate-700 border-slate-200'}`}>
									{a.is_enabled ? 'Enabled' : 'Disabled'}
								</span>
							</td>
							<td class="py-2 flex items-center gap-2">
								{#if a.analyzer_type === 'transcription' || a.analyzer_type === 'vision_tags' || a.analyzer_type === 'embedding'}
									<button type="button" class="h-8 px-2 inline-flex items-center justify-center rounded-md border border-indigo-200 text-indigo-700 hover:bg-indigo-50 text-xs font-medium" onclick={() => openAnalyzerTest(a)}>
										Test
									</button>
								{/if}
								<button type="button" class="h-8 w-8 inline-flex items-center justify-center rounded-md border border-slate-200 text-slate-600 hover:bg-slate-100" aria-label="Edit analyzer" onclick={() => openEditAnalyzer(a)}>
									<svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 20h9" /><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4Z" /></svg>
								</button>
								<button type="button" class={`h-8 w-8 inline-flex items-center justify-center rounded-md border ${a.is_enabled ? 'border-amber-200 text-amber-700 hover:bg-amber-50' : 'border-emerald-200 text-emerald-700 hover:bg-emerald-50'}`} aria-label={a.is_enabled ? 'Disable analyzer' : 'Enable analyzer'} onclick={() => toggleAnalyzer(a)}>
									<svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2"><path d="M20 6L9 17l-5-5" /></svg>
								</button>
								<button type="button" class="h-8 w-8 inline-flex items-center justify-center rounded-md border border-rose-200 text-rose-600 hover:bg-rose-50" aria-label="Delete analyzer" onclick={() => openDeleteAnalyzerConfirm(a)}>
									<svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 6h18" /><path d="M8 6V4h8v2" /><path d="M19 6l-1 14H6L5 6" /></svg>
								</button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	</div>
</div>

<ProviderModal
	open={providerModalOpen}
	mode={providerMode}
	provider={editingProvider}
	providerTypes={providerTypes}
	onClose={() => (providerModalOpen = false)}
	onSaved={onProviderSaved}
/>

<Modal open={analyzerModalOpen} title={analyzerMode === 'create' ? 'Add Analyzer' : 'Edit Analyzer'} description="Provider-first analyzer setup with provider-driven rules." widthClass="max-w-2xl" onClose={() => (analyzerModalOpen = false)}>
	<div class="space-y-4">
		{#if analyzerError}
			<div class="p-3 rounded bg-red-50 text-red-700 text-sm">{analyzerError}</div>
		{/if}
		<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
			<div>
				<label class="block text-xs font-medium mb-1">Provider</label>
				<select bind:value={analyzerProviderId} class="input-base" required onchange={onAnalyzerProviderChange}>
					{#each providers as p}
						<option value={p.id}>{p.name} ({providerTypeLabel(p.provider_type)})</option>
					{/each}
				</select>
				{#if selectedAnalyzerProviderType === 'deepgram'}
					<div class="mt-1 text-xs text-sky-600">Deepgram providers are locked to `transcription` analyzer type.</div>
				{/if}
			</div>
			<div>
				<label class="block text-xs font-medium mb-1">Analyzer Type</label>
				{#if analyzerTypeLockedByProvider}
					<div class="input-base bg-slate-50 text-slate-700">{analyzerLabel(analyzerType)} (provider-locked)</div>
				{:else}
					<select bind:value={analyzerType} class="input-base" onchange={onAnalyzerTypeChange}>
						<option value="transcription">transcription</option>
						<option value="vision_tags">vision_tags</option>
						<option value="embedding">embedding</option>
					</select>
				{/if}
			</div>
			<div>
				<label class="block text-xs font-medium mb-1">Model Name</label>
				<input type="text" bind:value={analyzerModelName} required={analyzerModelRequired} disabled={analyzerModelDisabled} class="input-base disabled:bg-slate-100 disabled:text-slate-500" placeholder={selectedAnalyzerProviderType === 'deepgram' ? 'Optional (defaults to nova-3)' : 'Model name'} />
				<div class="mt-1 text-xs text-slate-500">{analyzerModelHint}</div>
			</div>
			<div>
				<label class="block text-xs font-medium mb-1">Name</label>
				<input type="text" bind:value={analyzerName} class="input-base" required oninput={onAnalyzerNameInput} />
				<div class="mt-1 flex justify-between items-center gap-2">
					<span class="text-xs text-slate-500">Auto format: "{suggestedAnalyzerName()}"</span>
					<button type="button" class="text-xs px-2 py-1 rounded border border-slate-200 text-slate-600 hover:bg-slate-50" onclick={applyAnalyzerNameAutofill}>Autofill</button>
				</div>
			</div>
		</div>
		<div>
			<label class="block text-xs font-medium mb-1">Config JSON</label>
			<textarea rows="6" bind:value={analyzerConfigJson} class="input-base font-mono"></textarea>
		</div>
		<label class="text-sm flex items-center gap-2"><input type="checkbox" bind:checked={analyzerIsEnabled} /> Enabled</label>
		<div class="flex justify-end gap-2">
			<button type="button" class="btn-secondary text-sm" onclick={() => (analyzerModalOpen = false)}>Cancel</button>
			<button type="button" class="btn-primary text-sm disabled:opacity-60" disabled={analyzerSubmitting} onclick={submitAnalyzer}>
				{analyzerSubmitting ? 'Saving...' : analyzerMode === 'create' ? 'Create Analyzer' : 'Save Analyzer'}
			</button>
		</div>
	</div>
</Modal>

<Modal open={testModalOpen} title={`${testMetaByType[testRunType].label} Test: ${testAnalyzer?.name ?? ''}`} description={testMetaByType[testRunType].description} widthClass="max-w-4xl" onClose={closeAnalyzerTestModal}>
	<div class="space-y-4">
		<div class="flex flex-wrap items-center justify-between gap-3">
			<div class="text-xs text-slate-500">
				Provider: <span class="font-medium text-slate-700">{testAnalyzer?.provider_name ?? '-'}</span>
				<span class="mx-2">|</span>
				Model: <span class="font-medium text-slate-700">{testAnalyzer?.model_name ?? '-'}</span>
			</div>
			<div class="flex items-center gap-2">
				<button type="button" class="btn-secondary text-sm" onclick={() => (testTerminalEvents = [])} disabled={testRunning}>Clear</button>
				{#if testRunning}
					<button type="button" class="btn-secondary text-sm" onclick={stopAnalyzerTest}>Stop</button>
				{:else}
					<button type="button" class="btn-primary text-sm" onclick={startAnalyzerTest}>Run Test</button>
				{/if}
			</div>
		</div>
		{#if testError}
			<div class="p-3 rounded bg-red-50 text-red-700 text-sm">{testError}</div>
		{/if}
		{#if testSummary}
			<div class="grid grid-cols-3 gap-3 text-sm">
				<div class="rounded border border-emerald-200 bg-emerald-50 px-3 py-2">
					<div class="text-xs text-emerald-700">Successes</div>
					<div class="font-semibold text-emerald-800">{testSummary.successes}</div>
				</div>
				<div class="rounded border border-rose-200 bg-rose-50 px-3 py-2">
					<div class="text-xs text-rose-700">Failures</div>
					<div class="font-semibold text-rose-800">{testSummary.failures}</div>
				</div>
				<div class="rounded border border-amber-200 bg-amber-50 px-3 py-2">
					<div class="text-xs text-amber-700">Warnings</div>
					<div class="font-semibold text-amber-800">{testSummary.warnings}</div>
				</div>
			</div>
		{/if}
		<div class="rounded-xl border border-slate-800 bg-slate-950 p-3">
			<div class="text-xs uppercase tracking-widest text-slate-500 mb-2">Test Terminal</div>
			<div bind:this={testTerminalEl} class="h-80 overflow-y-auto font-mono text-xs leading-relaxed text-slate-100 space-y-2">
				{#if testTerminalEvents.length === 0}
					<div class="text-slate-500">No events yet. Start a test run to stream analyzer activity.</div>
				{:else}
					{#each testTerminalEvents as ev}
						<div>
							<div class="text-slate-400">[{prettyTimestamp(ev.timestamp)}] <span class="text-sky-300">{ev.event}</span></div>
							<div class="whitespace-pre-wrap break-words">{ev.message}</div>
							{#if ev.payload}
								<div class="text-slate-400 mt-0.5 whitespace-pre-wrap break-words">{JSON.stringify(ev.payload)}</div>
							{/if}
						</div>
					{/each}
				{/if}
			</div>
		</div>
	</div>
</Modal>

<Modal open={editorTestOpen} title="Editor Agent Test" description="Runs editor agent model connectivity checks and streams responses." widthClass="max-w-4xl" onClose={closeEditorTestModal}>
	<div class="space-y-4">
		<div class="flex flex-wrap items-center justify-between gap-3">
			<div class="text-xs text-slate-500">
				Provider: <span class="font-medium text-slate-700">{providers.find((p: any) => p.id === editorConfig.provider_id)?.name ?? '-'}</span>
				<span class="mx-2">|</span>
				Model: <span class="font-medium text-slate-700">{editorConfig.model_name || '-'}</span>
			</div>
			<div class="flex items-center gap-2">
				<button type="button" class="btn-secondary text-sm" onclick={() => (editorTestEvents = [])} disabled={editorTestRunning}>Clear</button>
				{#if editorTestRunning}
					<button type="button" class="btn-secondary text-sm" onclick={stopEditorTest}>Stop</button>
				{:else}
					<button type="button" class="btn-primary text-sm" onclick={startEditorTest}>Run Test</button>
				{/if}
			</div>
		</div>
		{#if editorTestError}
			<div class="p-3 rounded bg-red-50 text-red-700 text-sm">{editorTestError}</div>
		{/if}
		{#if editorTestSummary}
			<div class="grid grid-cols-3 gap-3 text-sm">
				<div class="rounded border border-emerald-200 bg-emerald-50 px-3 py-2">
					<div class="text-xs text-emerald-700">Successes</div>
					<div class="font-semibold text-emerald-800">{editorTestSummary.successes}</div>
				</div>
				<div class="rounded border border-rose-200 bg-rose-50 px-3 py-2">
					<div class="text-xs text-rose-700">Failures</div>
					<div class="font-semibold text-rose-800">{editorTestSummary.failures}</div>
				</div>
				<div class="rounded border border-amber-200 bg-amber-50 px-3 py-2">
					<div class="text-xs text-amber-700">Warnings</div>
					<div class="font-semibold text-amber-800">{editorTestSummary.warnings}</div>
				</div>
			</div>
		{/if}
		<div class="rounded-xl border border-slate-800 bg-slate-950 p-3">
			<div class="text-xs uppercase tracking-widest text-slate-500 mb-2">Editor Test Terminal</div>
			<div bind:this={editorTestTerminalEl} class="h-80 overflow-y-auto font-mono text-xs leading-relaxed text-slate-100 space-y-2">
				{#if editorTestEvents.length === 0}
					<div class="text-slate-500">No events yet. Start a test run to stream editor activity.</div>
				{:else}
					{#each editorTestEvents as ev}
						<div>
							<div class="text-slate-400">[{prettyTimestamp(ev.timestamp)}] <span class="text-sky-300">{ev.event}</span></div>
							<div class="whitespace-pre-wrap break-words">{ev.message}</div>
							{#if ev.payload}
								<div class="text-slate-400 mt-0.5 whitespace-pre-wrap break-words">{JSON.stringify(ev.payload)}</div>
							{/if}
						</div>
					{/each}
				{/if}
			</div>
		</div>
	</div>
</Modal>

<ConfirmModal
	open={confirmDeleteOpen}
	title={confirmDeleteType === 'provider' ? 'Delete model provider?' : 'Delete analyzer?'}
	message={confirmDeleteType === 'provider'
		? `This will delete provider "${confirmDeleteName}". This is only allowed when no analyzers are using it.`
		: `This will disable analyzer "${confirmDeleteName}".`}
	confirmText={confirmDeleteType === 'provider' ? 'Delete Provider' : 'Delete Analyzer'}
	danger={true}
	pending={confirmDeleteSubmitting}
	onCancel={closeDeleteConfirm}
	onConfirm={confirmDelete}
/>
