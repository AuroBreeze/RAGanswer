<script lang="ts">
	import { onMount } from 'svelte';
	import fragmentSource from './jelly-orb.frag.glsl?raw';

	let canvas: HTMLCanvasElement | undefined = $state();
	let error = $state('');

	const vertexSource = `#version 300 es
void main(){vec2 p=vec2((gl_VertexID<<1)&2,gl_VertexID&2);gl_Position=vec4(p*2.-1.,0.,1.);}`;

	const fragmentHeader = `#version 300 es
precision highp float;
precision highp int;
uniform vec3 iResolution;
uniform float iTime;
out vec4 outColor;
`;

	const fragmentFooter = `
void main(){vec4 c=vec4(0.);mainImage(c,gl_FragCoord.xy);outColor=c;}
`;

	onMount(() => {
		const element = canvas;
		if (!element) return;

		const el: HTMLCanvasElement = element;
		const context = el.getContext('webgl2', { antialias: false, alpha: true });
		if (!context) {
			error = '需要 WebGL2 支持。';
			return;
		}
		const gl: WebGL2RenderingContext = context;

		function compile(type: number, source: string) {
			const shader = gl.createShader(type);
			if (!shader) return null;

			gl.shaderSource(shader, source);
			gl.compileShader(shader);

			if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
				error = gl.getShaderInfoLog(shader) ?? '着色器编译失败';
				gl.deleteShader(shader);
				return null;
			}

			return shader;
		}

		const vertexShader = compile(gl.VERTEX_SHADER, vertexSource);
		const fragmentShader = compile(
			gl.FRAGMENT_SHADER,
			fragmentHeader + fragmentSource + fragmentFooter
		);

		if (!vertexShader || !fragmentShader) return;

		const program = gl.createProgram();
		if (!program) {
			error = '无法创建 WebGL 程序。';
			return;
		}

		gl.attachShader(program, vertexShader);
		gl.attachShader(program, fragmentShader);
		gl.linkProgram(program);

		if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
			error = gl.getProgramInfoLog(program) ?? '着色器链接失败';
			return;
		}

		gl.useProgram(program);

		const resolution = gl.getUniformLocation(program, 'iResolution');
		const time = gl.getUniformLocation(program, 'iTime');
		const vao = gl.createVertexArray();
		gl.bindVertexArray(vao);

		function resize() {
			const dpr = Math.min(window.devicePixelRatio || 1, 1.5);
			const width = Math.max(1, Math.floor(el.clientWidth * dpr));
			const height = Math.max(1, Math.floor(el.clientHeight * dpr));

			if (el.width !== width || el.height !== height) {
				el.width = width;
				el.height = height;
			}
		}

		const startedAt = performance.now();
		let frameId = 0;

		function frame() {
			resize();
			gl.viewport(0, 0, el.width, el.height);
			gl.uniform3f(resolution, el.width, el.height, 1);
			gl.uniform1f(time, (performance.now() - startedAt) / 1000 + 2);
			gl.drawArrays(gl.TRIANGLES, 0, 3);
			frameId = requestAnimationFrame(frame);
		}

		frameId = requestAnimationFrame(frame);

		return () => {
			cancelAnimationFrame(frameId);
			gl.deleteProgram(program);
			gl.deleteShader(vertexShader);
			gl.deleteShader(fragmentShader);
			gl.deleteVertexArray(vao);
		};
	});
</script>

<div class="relative h-full w-full overflow-hidden">
	<canvas bind:this={canvas} class="block h-full w-full" aria-label="果冻玻璃球体"></canvas>

	{#if error}
		<pre
			class="absolute inset-0 overflow-auto whitespace-pre-wrap bg-zinc-950 p-4 font-mono text-xs text-zinc-200"
		>{error}</pre>
	{/if}
</div>
