export interface Citation {
	document_id: string;
	title: string;
	version: string | null;
	section: string | null;
	page: number | null;
	excerpt: string;
	url: string | null;
}

export interface ChatAnswer {
	answer: string;
	citations: Citation[];
}

export async function askQuestion(question: string): Promise<ChatAnswer> {
	const response = await fetch('/api/v1/chat', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ question })
	});
	if (response.status === 401 || response.status === 403) {
		throw new Error('当前身份无法使用问答，请先登录。');
	}
	if (!response.ok) throw new Error('问答服务暂时不可用，请稍后重试。');
	const data = (await response.json().catch(() => null)) as Partial<ChatAnswer> | null;
	if (typeof data?.answer === 'string' && Array.isArray(data.citations)) {
		return { answer: data.answer, citations: data.citations };
	}
	throw new Error('问答服务返回了无法读取的响应。');
}

export function citationHref(url: string | null): string | null {
	if (!url?.startsWith('/')) return null;
	const origin = 'https://raganswer.invalid';
	if (!URL.canParse(url, origin)) return null;
	const target = new URL(url, origin);
	return target.origin === origin ? `${target.pathname}${target.search}${target.hash}` : null;
}
