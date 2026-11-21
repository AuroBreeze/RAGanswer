<script lang="ts">
	import { onMount } from 'svelte';

	let { revealed = false }: { revealed?: boolean } = $props();

	const vertexSource = `#version 300 es
void main(){vec2 p=vec2((gl_VertexID<<1)&2,gl_VertexID&2);gl_Position=vec4(p*2.-1.,0.,1.);}`;

	const fragmentHeader = `#version 300 es
precision highp float;
precision highp int;
uniform vec3 iResolution;
uniform float iTime;
out vec4 outColor;
`;

	const fragmentSource = `#ifdef GL_ES
precision mediump float;
#endif

float cosRange(float amt, float range, float minimum) {
	return (((1.0 + cos(radians(amt))) * 0.5) * range) + minimum;
}

void mainImage( out vec4 fragColor, in vec2 fragCoord )
{
    const int zoom = 40;
    const float brightness = 0.975;
	float time = iTime * 1.25;
	vec2 uv = fragCoord.xy / iResolution.xy;
	vec2 p  = (2.0*fragCoord.xy-iResolution.xy)/max(iResolution.x,iResolution.y);
	float ct = cosRange(time*5.0, 3.0, 1.1);
	float xBoost = cosRange(time*0.2, 5.0, 5.0);
	float yBoost = cosRange(time*0.1, 10.0, 5.0);
	float fScale = cosRange(time * 15.5, 1.25, 0.5);

	for(int i=1;i<zoom;i++) {
		float _i = float(i);
		vec2 newp=p;
		newp.x+=0.25/_i*sin(_i*p.y+time*cos(ct)*0.5/20.0+0.005*_i)*fScale+xBoost;
		newp.y+=0.25/_i*sin(_i*p.x+time*ct*0.3/40.0+0.03*float(i+15))*fScale+yBoost;
		p=newp;
	}

	vec3 col=vec3(0.5*sin(3.0*p.x)+0.5,0.5*sin(3.0*p.y)+0.5,sin(p.x+p.y));
	col *= brightness;

	float vigAmt = 5.0;
	float vignette = (1.-vigAmt*(uv.y-.5)*(uv.y-.5))*(1.-vigAmt*(uv.x-.5)*(uv.x-.5));
	float extrusion = (col.x + col.y + col.z) / 4.0;
    extrusion *= 1.5;
    extrusion *= vignette;

	fragColor = vec4(col, extrusion);
}`;

	const fragmentFooter = `
void main(){vec4 c=vec4(0.);mainImage(c,gl_FragCoord.xy);outColor=c;}
`;

	let canvas: HTMLCanvasElement | undefined = $state();
	let start: (() => void) | null = null;
	let stop: (() => void) | null = null;

	$effect(() => {
		if (revealed) start?.();
		else stop?.();
	});

	onMount(() => {
		const element = canvas;
		if (!element) return;
		const el: HTMLCanvasElement = element;

		const context = el.getContext('webgl2', {
			antialias: false,
			alpha: true,
			premultipliedAlpha: false
		});
		if (!context) return;
		const gl: WebGL2RenderingContext = context;

		function compile(type: number, source: string) {
			const shader = gl.createShader(type);
			if (!shader) return null;

			gl.shaderSource(shader, source);
			gl.compileShader(shader);

			if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
				console.error(gl.getShaderInfoLog(shader));
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
		if (!program) return;

		gl.attachShader(program, vertexShader);
		gl.attachShader(program, fragmentShader);
		gl.linkProgram(program);

		if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
			console.error(gl.getProgramInfoLog(program));
			return;
		}

		gl.useProgram(program);

		const resolution = gl.getUniformLocation(program, 'iResolution');
		const time = gl.getUniformLocation(program, 'iTime');
		const vao = gl.createVertexArray();
		gl.bindVertexArray(vao);

		let frameId = 0;
		let startedAt = 0;
		let running = false;

		function resize() {
			const dpr = Math.min(window.devicePixelRatio || 1, 1.25);
			const width = Math.max(1, Math.floor(el.clientWidth * dpr));
			const height = Math.max(1, Math.floor(el.clientHeight * dpr));

			if (el.width !== width || el.height !== height) {
				el.width = width;
				el.height = height;
			}
		}

		function frame() {
			resize();
			gl.viewport(0, 0, el.width, el.height);
			gl.uniform3f(resolution, el.width, el.height, 1);
			gl.uniform1f(time, (performance.now() - startedAt) / 1000);
			gl.drawArrays(gl.TRIANGLES, 0, 3);
			frameId = requestAnimationFrame(frame);
		}

		start = () => {
			if (running) return;
			running = true;
			if (!startedAt) startedAt = performance.now();
			frameId = requestAnimationFrame(frame);
		};

		stop = () => {
			running = false;
			cancelAnimationFrame(frameId);
		};

		if (revealed) start();

		return () => {
			stop?.();
			gl.deleteProgram(program);
			gl.deleteShader(vertexShader);
			gl.deleteShader(fragmentShader);
			gl.deleteVertexArray(vao);
		};
	});
</script>

<canvas
	bind:this={canvas}
	class={`pointer-events-none fixed inset-0 z-0 h-full w-full transition-opacity duration-[1600ms] ease-out ${
		revealed ? 'opacity-100' : 'opacity-0'
	}`}
	aria-hidden="true"
></canvas>
