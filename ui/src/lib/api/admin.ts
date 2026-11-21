export interface KnowledgeDocument {
	document_id: string;
	source_type: 'web' | 'pdf';
	title: string;
	version: string | null;
	document_status: string;
	source_status: string;
	parse_id: string | null;
	job_status: 'queued' | 'indexing' | 'ready' | 'failed' | null;
	indexed_chunk_count: number | null;
	updated_at: string | null;
	searchable: boolean;
	error: { code: string; message: string } | null;
}

async function request(token: string, path: string, options: RequestInit = {}): Promise<Response> {
	const headers = new Headers(options.headers);
	headers.set('Authorization', `Bearer ${token}`);
	const response = await fetch(`/internal/v1/admin/${path}`, { ...options, headers });
	if (response.ok) return response;
	if (response.status === 401 || response.status === 403) throw new Error('管理令牌无效。');
	const body = (await response.json().catch(() => null)) as {
		error?: { message?: string };
		detail?: string;
	} | null;
	throw new Error(body?.error?.message ?? body?.detail ?? '管理服务暂时不可用。');
}

export async function listDocuments(token: string): Promise<KnowledgeDocument[]> {
	const response = await request(token, 'documents');
	const data = (await response.json()) as { items: KnowledgeDocument[] };
	return data.items;
}

export async function uploadArtifact(token: string, file: File): Promise<void> {
	await request(token, 'documents', {
		method: 'POST',
		headers: { 'Content-Type': 'application/x-ndjson' },
		body: file
	});
}

export async function approveArtifact(token: string, parseId: string): Promise<void> {
	await request(token, `parses/${encodeURIComponent(parseId)}/approve`, { method: 'POST' });
}

export async function revokeDocument(token: string, documentId: string): Promise<void> {
	await request(token, `documents/${encodeURIComponent(documentId)}/revoke`, { method: 'POST' });
}
