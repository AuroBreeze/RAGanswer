<script lang="ts">
	import { onMount } from 'svelte';

	let { ontap }: { ontap?: () => void } = $props();

	const WID = 0.42;
	const SPEED = 0.42;
	const INS = 0.06;
	// 单次脉冲：波长大于脉冲包络，避免一次触摸出现多圈波纹
	const LAM = 1.1;

	const vertexShader = 'attribute vec2 a;void main(){gl_Position=vec4(a,0.,1.);}';

	const fragmentShader = `precision highp float;
uniform vec2 uR;
uniform vec4 uP[8];
uniform sampler2D uG;
uniform float uT;

const float W=${WID};
const float LAM=${LAM};

vec3 pal(float t){
	vec3 A=vec3(.04,.42,.28);
	vec3 B=vec3(.48,.90,.64);
	return mix(A,B,smoothstep(0.,1.,t))+vec3(.05,.04,.0)*sin(3.14159*t);
}

void main(){
	vec2 p=gl_FragCoord.xy/uR.y;
	float asp=uR.x/uR.y;
	vec3 col=mix(vec3(.004,.014,.010),vec3(.035,.080,.055),p.y);
	float wm=smoothstep(0.,.16,p.y);
	vec3 L=normalize(vec3(-.4,.55,.75));
	vec3 Hh=normalize(L+vec3(0.,0.,1.));

	for(int i=0;i<8;i++){
		float r=uP[i].z;
		if(r<0.)continue;
		float d=length(p-uP[i].xy)+1e-4;
		float k=(r-d)/W;
		float fade=exp(-r*.35);
		col=mix(col,pal(.3),exp(-d*d*30.)*exp(-r*5.)*.7*wm);
		if(k<0.||k>2.6)continue;
		float e=smoothstep(0.,.1,k)*exp(-k*1.3);
		float ph=6.2832*(r-d)/LAM;
		vec2 dir=(p-uP[i].xy)/d;
		vec3 n=normalize(vec3(-dir*cos(ph)*e*1.3,1.));
		float df=dot(n,L);
		float sp=pow(max(dot(n,Hh),0.),40.);
		vec3 base=pal(.5+.5*sin(k*1.6+r*1.6+uP[i].w));
		vec3 lit=base*(.4+.95*df)+sp*e*vec3(.9,1.,.92)*.6;
		col=mix(col,lit,clamp(e*1.1,0.,1.)*.9*fade*wm);
	}

	vec4 g=texture2D(uG,vec2(p.x/asp,.5));
	float q=g.r/max(g.a,.001);
	float lt=g.g/max(g.a,.001);

	for(int l=0;l<4;l++){
		float fl=3.-float(l);
		float hm=.55+.45*(.6*sin(p.x*(7.+fl*2.5)+uT*(1.1+fl*.35)+fl*1.7)+.4*sin(p.x*(13.-fl*2.)-uT*(.8+fl*.3)+fl));
		float Hl=(.05+fl*.04)*hm*(.35+.65*g.a);
		float prof=exp(-pow(max(p.y,0.)/max(Hl,.001),1.6));
		vec3 c=pal(.5+.5*sin(uT*.9+q*6.2832+fl*.9))*lt;
		col=mix(col,c*(1.05-.12*fl),g.a*prof*(.95-.2*fl));
	}

	col+=pal(.5+.5*sin(uT*.9+q*6.2832))*lt*g.a*exp(-p.y/.3)*.1;
	gl_FragColor=vec4(col,1.);
}`;

	type Wave = { x: number; y: number; r: number; seed: number };

	let canvas: HTMLCanvasElement;

	onMount(() => {
		const maybeContext = canvas.getContext('webgl', { alpha: false, antialias: false });
		if (!maybeContext) return;
		const context: WebGLRenderingContext = maybeContext;

		function compileShader(type: number, source: string) {
			const shader = context.createShader(type);
			if (!shader) return null;
			context.shaderSource(shader, source);
			context.compileShader(shader);
			if (!context.getShaderParameter(shader, context.COMPILE_STATUS)) {
				console.error(context.getShaderInfoLog(shader));
				context.deleteShader(shader);
				return null;
			}
			return shader;
		}

		const vs = compileShader(context.VERTEX_SHADER, vertexShader);
		const fs = compileShader(context.FRAGMENT_SHADER, fragmentShader);
		if (!vs || !fs) return;

		const program = context.createProgram();
		if (!program) return;
		context.attachShader(program, vs);
		context.attachShader(program, fs);
		context.linkProgram(program);
		context.useProgram(program);

		const buffer = context.createBuffer();
		context.bindBuffer(context.ARRAY_BUFFER, buffer);
		context.bufferData(
			context.ARRAY_BUFFER,
			new Float32Array([-1, -1, 1, -1, -1, 1, 1, 1]),
			context.STATIC_DRAW
		);

		const a = context.getAttribLocation(program, 'a');
		context.enableVertexAttribArray(a);
		context.vertexAttribPointer(a, 2, context.FLOAT, false, 0, 0);

		const uR = context.getUniformLocation(program, 'uR');
		const uP = context.getUniformLocation(program, 'uP');
		const uT = context.getUniformLocation(program, 'uT');
		context.uniform1i(context.getUniformLocation(program, 'uG'), 0);

		const N = 512;
		const locked = new Float32Array(N);
		const colorQ = new Float32Array(N);
		const colorLt = new Float32Array(N);
		const pixels = new Uint8Array(N * 4);

		const texture = context.createTexture();
		context.bindTexture(context.TEXTURE_2D, texture);
		for (const [key, value] of [
			[context.TEXTURE_MIN_FILTER, context.LINEAR],
			[context.TEXTURE_MAG_FILTER, context.LINEAR],
			[context.TEXTURE_WRAP_S, context.CLAMP_TO_EDGE],
			[context.TEXTURE_WRAP_T, context.CLAMP_TO_EDGE]
		]) {
			context.texParameteri(context.TEXTURE_2D, key, value);
		}

		let width = 1;
		let height = 1;
		let aspect = 1;

		function resize() {
			const dpr = Math.min(window.devicePixelRatio || 1, 1.5);
			width = Math.floor(window.innerWidth * dpr);
			height = Math.floor(window.innerHeight * dpr);
			canvas.width = width;
			canvas.height = height;
			aspect = width / height;
			context.viewport(0, 0, width, height);
		}

		function smoothstep(a: number, b: number, x: number) {
			const t = Math.min(1, Math.max(0, (x - a) / (b - a)));
			return t * t * (3 - 2 * t);
		}

		const waves: Wave[] = [];

		function addWave(clientX: number, clientY: number) {
			const rect = canvas.getBoundingClientRect();
			if (!rect.width || !rect.height) return;
			if (waves.length >= 8) waves.shift();
			waves.push({
				x: ((clientX - rect.left) / rect.width) * aspect,
				y: Math.max(INS, 1 - (clientY - rect.top) / rect.height),
				r: 0,
				seed: Math.random() * 6.28
			});
		}

		function handlePointerDown(event: PointerEvent) {
			if (event.button !== 0) return;
			addWave(event.clientX, event.clientY);
			ontap?.();
		}

		resize();
		window.addEventListener('resize', resize);
		window.addEventListener('pointerdown', handlePointerDown, { passive: true });

		const params = new Float32Array(32);
		let last = performance.now();
		let frameId = 0;

		function frame(now: number) {
			const dt = Math.min(0.033, (now - last) / 1000);
			last = now;

			for (const wave of waves) wave.r += SPEED * dt;
			while (waves.length && waves[0].r > Math.hypot(aspect, 1) + 2.4) waves.shift();

			for (const wave of waves) {
				const fade = Math.exp(-wave.r * 0.35);
				for (let i = 0; i < N; i++) {
					const x = ((i + 0.5) / N) * aspect;
					const d = Math.hypot(x - wave.x, INS - wave.y);
					const k = (wave.r - d) / WID;
					if (k < 0 || k > 2.6) continue;
					const e = smoothstep(0, 0.1, k) * Math.exp(-k * 1.3);
					const amount = Math.min(1, e * fade * 7 * dt);
					if (amount <= 0) continue;
					const lt = 0.8 + 0.2 * Math.sin((6.2832 * (wave.r - d)) / LAM);
					const q = 0.5 + 0.5 * Math.sin(k * 1.6 + wave.r * 1.6 + wave.seed);
					const mix = amount * (1 - locked[i]);
					colorQ[i] += q * mix;
					colorLt[i] += lt * mix;
					locked[i] += mix;
				}
			}

			if (waves.length) {
				for (let i = 0; i < N; i++) {
					if (locked[i] >= 0.999) continue;
					for (const j of [i - 1, i + 1]) {
						if (j < 0 || j >= N || locked[j] < 0.02) continue;
						const flow = 0.2 * Math.max(0, locked[j] - locked[i]) * (1 - locked[i]);
						if (flow <= 0) continue;
						colorQ[i] += (flow * colorQ[j]) / locked[j];
						colorLt[i] += (flow * colorLt[j]) / locked[j];
						locked[i] += flow;
					}
				}
			}

			for (let i = 0; i < N; i++) {
				pixels[i * 4] = colorQ[i] * 255;
				pixels[i * 4 + 1] = colorLt[i] * 255;
				pixels[i * 4 + 2] = 0;
				pixels[i * 4 + 3] = locked[i] * 255;
			}
			context.texImage2D(context.TEXTURE_2D, 0, context.RGBA, N, 1, 0, context.RGBA, context.UNSIGNED_BYTE, pixels);

			params.fill(0);
			for (let i = 0; i < 8; i++) params[i * 4 + 2] = -1;
			waves.forEach((wave, i) => {
				params[i * 4] = wave.x;
				params[i * 4 + 1] = wave.y;
				params[i * 4 + 2] = wave.r;
				params[i * 4 + 3] = wave.seed;
			});

			context.uniform2f(uR, width, height);
			context.uniform4fv(uP, params);
			context.uniform1f(uT, now / 1000);
			context.drawArrays(context.TRIANGLE_STRIP, 0, 4);
			frameId = requestAnimationFrame(frame);
		}

		frameId = requestAnimationFrame(frame);

		return () => {
			cancelAnimationFrame(frameId);
			window.removeEventListener('resize', resize);
			window.removeEventListener('pointerdown', handlePointerDown);
			context.deleteTexture(texture);
			context.deleteProgram(program);
			context.deleteShader(vs);
			context.deleteShader(fs);
			context.deleteBuffer(buffer);
		};
	});
</script>

<canvas
	bind:this={canvas}
	class="pointer-events-none fixed inset-0 z-0 h-full w-full"
	aria-hidden="true"
></canvas>
