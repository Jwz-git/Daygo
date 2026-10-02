import { Color, ColorManagement, DoubleSide, InstancedBufferAttribute, InstancedBufferGeometry, LinearSRGBColorSpace, Mesh, PerspectiveCamera, PlaneGeometry, Scene, ShaderMaterial, Vector2, Vector3, WebGLRenderer } from 'three';

ColorManagement.enabled = false;

/* ------------------------------------------------------------------ */
/*  Sky palettes — one per time of day. Colors are raw sRGB.           */
/* ------------------------------------------------------------------ */
const hex = (h) => { const c = new Color(h); return [c.r, c.g, c.b]; };
const P = (top, mid, hor, sun, sunY, sunI, stars, ground, sunX = 0.5) => ({ top: hex(top), mid: hex(mid), hor: hex(hor), sun: hex(sun), sunY, sunI, stars, ground, sunX });

export const SKIES = {
  dawn:      P('#06071a', '#1a1840', '#ff6a3d', '#ffb26a', 0.02, 1.0, 0.55, 1.0),
  morning:   P('#0b1030', '#262a5c', '#c9765a', '#ffd9a8', 0.42, 0.42, 0.12, 0.6, 0.86),
  forenoon:  P('#0b1230', '#18214a', '#34467f', '#cfe0ff', 0.8, 0.22, 0.0, 0.15, 0.9),
  noon:      P('#ebe6db', '#f2eee6', '#fbf6ea', '#ffffff', 0.95, 0.45, 0.0, 0.0, 0.6),
  afternoon: P('#efe5d3', '#f4ebdd', '#ffdcae', '#ffe2b8', 0.55, 0.5, 0.0, 0.0, 0.18),
  dusk:      P('#0f0b28', '#35204f', '#ff6b3d', '#ff8a4c', 0.0, 1.0, 0.25, 1.0, 0.72),
  evening:   P('#07071a', '#141331', '#45254f', '#ff7a4c', -0.35, 0.35, 0.6, 0.8, 0.8),
  night:     P('#040407', '#08080f', '#12122a', '#8c7cff', -0.6, 0.0, 0.9, 0.5),
  midnight:  P('#030305', '#060609', '#0e0e1e', '#8c7cff', -0.8, 0.0, 1.0, 0.35),
};

export function mixSky(a, b, t) {
  const m = (x, y) => x.map((v, i) => v + (y[i] - v) * t);
  const n = (x, y) => x + (y - x) * t;
  return { top: m(a.top, b.top), mid: m(a.mid, b.mid), hor: m(a.hor, b.hor), sun: m(a.sun, b.sun), sunX: n(a.sunX, b.sunX), sunY: n(a.sunY, b.sunY), sunI: n(a.sunI, b.sunI), stars: n(a.stars, b.stars), ground: n(a.ground, b.ground) };
}

/* ------------------------------------------------------------------ */
const skyVert = /* glsl */ `
varying vec2 vUv;
void main(){ vUv = uv; gl_Position = vec4(position.xy, 0.9999, 1.0); }`;

const skyFrag = /* glsl */ `
precision highp float;
varying vec2 vUv;
uniform vec3 uTop, uMid, uHor, uSun;
uniform float uSunX, uSunY, uSunI, uStars, uGround, uTime;
uniform vec2 uRes, uMouse;

float hash(vec2 p){ p = fract(p*vec2(123.34, 456.21)); p += dot(p, p+45.32); return fract(p.x*p.y); }
float noise(vec2 p){ vec2 i=floor(p), f=fract(p); vec2 u=f*f*(3.-2.*f);
  return mix(mix(hash(i),hash(i+vec2(1,0)),u.x), mix(hash(i+vec2(0,1)),hash(i+vec2(1,1)),u.x), u.y); }
float fbm(vec2 p){ float v=0., a=.5; for(int i=0;i<4;i++){ v+=a*noise(p); p*=2.03; a*=.5; } return v; }

void main(){
  vec2 uv = vUv;
  float aspect = uRes.x / uRes.y;
  float H = 0.30;                       // horizon line
  float y = uv.y;

  // Sky gradient
  vec3 col = mix(uHor, uMid, smoothstep(H, H + 0.42, y));
  col = mix(col, uTop, smoothstep(H + 0.25, 1.05, y));

  // Slow atmospheric drift
  vec2 q = vec2(uv.x * aspect * 1.4, uv.y * 2.2);
  float n = fbm(q + vec2(uTime * 0.012, -uTime * 0.006) + 0.6 * noise(q * 1.7 - uTime * 0.01));
  col += (n - 0.5) * 0.07 * (0.6 + 0.4 * smoothstep(0.0, 1.0, y));

  // Sun
  vec2 sp = vec2(uSunX + uMouse.x * 0.015, H + uSunY * 0.62 + uMouse.y * 0.01);
  vec2 d = uv - sp; d.x *= aspect;
  float r = length(d);
  float glow = exp(-r * 4.2) * 0.55 + exp(-r * 13.0) * 0.45;
  col += uSun * glow * uSunI * 0.85;
  // sun disc sliced into frames (the Daygo mark)
  float disc = 1.0 - smoothstep(0.088, 0.092, r);
  float rel = (uv.y - H) / 0.09;
  float slit = 1.0;
  for (int i = 0; i < 4; i++) {
    float c = 0.12 + float(i) * 0.17;
    float w = 0.028 + float(i) * 0.016;
    slit *= smoothstep(w, w + 0.006, abs(rel - c));
  }
  float aboveH = smoothstep(H - 0.002, H + 0.002, uv.y);
  col = mix(col, mix(uSun, vec3(1.0), 0.25), disc * uSunI * mix(1.0, slit, uGround) * mix(1.0, aboveH, uGround));

  // Ground: a calm dark sea below the horizon with the sun's reflection
  float below = 1.0 - smoothstep(H - 0.0015, H + 0.0015, y);
  vec3 ground = mix(uTop * 0.7, uMid * 0.55, smoothstep(0.0, H, y));
  float streak = exp(-abs(d.x) * 9.0) * smoothstep(0.0, H, y) * (0.6 + 0.4 * noise(vec2(uv.x * 60.0, uv.y * 240.0 - uTime * 0.6)));
  ground += uSun * streak * uSunI * 0.55;
  col = mix(col, ground, below * uGround);
  col += uHor * exp(-abs(y - H) * 90.0) * 0.35 * uGround;

  // Stars
  vec2 g = vec2(uv.x * aspect, uv.y) * 140.0;
  vec2 id = floor(g); vec2 f = fract(g) - 0.5;
  float h = hash(id);
  float tw = 0.55 + 0.45 * sin(uTime * (1.0 + h * 3.0) + h * 40.0);
  float s = step(0.982, h) * (1.0 - smoothstep(0.0, 0.09, length(f + (vec2(hash(id + 3.1), hash(id + 7.7)) - 0.5) * 0.6))) * tw;
  col += vec3(1.0, 0.96, 0.9) * s * uStars * smoothstep(H, H + 0.25, y) * 0.9;

  // Vignette + grain
  vec2 vv = uv - 0.5; vv.x *= aspect * 0.6;
  col *= 1.0 - dot(vv, vv) * 0.35;
  col += (hash(mod(floor(gl_FragCoord.xy), 512.0) * 0.013 + fract(uTime * 7.0)) - 0.5) * 0.028;

  gl_FragColor = vec4(col, 1.0);
}`;

const frameVert = /* glsl */ `
attribute vec4 aRand;
attribute vec3 aScatter;
attribute vec3 aStar;
attribute float aT;
attribute float aLane;
attribute vec3 aColor;
attribute float aKind;
uniform float uTime, uMorph, uExit, uStar, uWidth, uPulse;
uniform vec2 uMouse;
varying vec2 vUv; varying vec3 vColor; varying float vFill, vAlpha, vKind, vStar, vDepth, vBright;

mat3 rotX(float a){ float c=cos(a), s=sin(a); return mat3(1.,0.,0., 0.,c,s, 0.,-s,c); }
mat3 rotY(float a){ float c=cos(a), s=sin(a); return mat3(c,0.,-s, 0.,1.,0., s,0.,c); }
mat3 rotZ(float a){ float c=cos(a), s=sin(a); return mat3(c,s,0., -s,c,0., 0.,0.,1.); }
float ease(float x){ float y = 2. - 2. * x; return x < .5 ? 4. * x * x * x : 1. - y * y * y * .5; }

void main(){
  vec3 v = position;

  // 1 · scattered captures drifting like dust
  vec3 sp = aScatter;
  sp.x += sin(uTime * .13 + aRand.y * 6.28) * .28;
  sp.y += cos(uTime * .11 + aRand.z * 6.28) * .22 + uPulse * (aRand.z - .5) * .6;
  vec2 dm = sp.xy - uMouse; float dl = length(dm);
  sp.xy += (dm / (dl + 1e-4)) * exp(-dl * dl * .8) * 1.1 * (1. - uMorph);
  mat3 sr = rotZ((aRand.w - .5) * 1.2 + uTime * .08 * (aRand.y - .5)) * rotY((aRand.z - .5) * 1.6 + uTime * .15 * (aRand.w - .5)) * rotX((aRand.y - .5) * 1.1);
  float sS = .18 + aRand.w * .26;

  // 2 · the ribbon — a continuous timeline woven from frames
  float t = aT;
  float x = (t - .5) * uWidth;
  float theta = t * 6.2831 * .9 + uTime * .32;
  float baseY = sin(t * 3.1415 * 1.5 + uTime * .22) * .42 + .55;
  float lane = aLane * .52;
  vec3 rp = vec3(x, baseY + cos(theta) * lane, sin(theta) * lane);
  float wx = (x - uMouse.x) * .9; float wave = exp(-wx * wx);
  rp.y += wave * .22 * sin(uTime * 2.2 + t * 24.);
  rp.z += wave * .4;
  mat3 rr = rotX(theta);
  float rS = .15;

  float p = ease(clamp(uMorph * 1.7 - (t * .55 + aRand.x * .15), 0., 1.));
  vec3 pos = mix(sp, rp, p);
  vec3 lv = mix(sr * (v * sS), rr * (v * rS), p);
  pos.y += uExit * uExit * 5.5;
  pos.z += uExit * 2.0;

  // 3 · night — every frame of the day becomes a star
  float q = ease(clamp(uStar * 1.4 - aRand.x * .4, 0., 1.));
  vec3 st = aStar + vec3(sin(uTime * .05 + aRand.y * 6.) * .15, 0., 0.);
  pos = mix(pos, st, q);
  lv = mix(lv, v * (.03 + aRand.w * aRand.w * .07), q);

  vec4 mv = viewMatrix * vec4(pos + lv, 1.);
  gl_Position = projectionMatrix * mv;
  vUv = uv; vColor = aColor; vFill = p; vKind = aKind; vStar = q;
  vDepth = clamp((-mv.z - 6.) / 18., 0., 1.);
  vAlpha = max(1. - smoothstep(.45, 1., uExit), q);
  vBright = (.25 + .75 * aRand.y * aRand.y) * (.6 + .4 * sin(uTime * (1. + aRand.z * 2.) + aRand.x * 30.));
}`;

const frameFrag = /* glsl */ `
precision highp float;
varying vec2 vUv; varying vec3 vColor; varying float vFill, vAlpha, vKind, vStar, vDepth, vBright;
float sdBox(vec2 p, vec2 b, float r){ vec2 q = abs(p) - b + r; return length(max(q, 0.)) + min(max(q.x, q.y), 0.) - r; }
float h1(float n){ return fract(sin(n) * 43758.5453); }
void main(){
  vec2 p = (vUv - .5) * vec2(1.6, 1.);
  float d = sdBox(p, vec2(.8, .5), .07);
  float aa = max(fwidth(d) * 1.2, 1e-4);
  float shape = 1. - smoothstep(-aa, aa, d);

  // a tiny screenshot: title bar + rows of content
  vec3 col = vec3(.065, .065, .085);
  col = mix(col, vec3(.12, .12, .15), step(.37, p.y));
  float yy = (.31 - p.y) / .105;
  float row = floor(yy);
  float inRow = step(0., yy) * step(fract(yy), .42) * step(row, 6.);
  float indent = floor(h1(row * 3.1 + vKind) * 3.) * .1;
  float len = .2 + h1(row * 7.3 + vKind * 1.7) * .95;
  float ln = inRow * step(-.68 + indent, p.x) * step(p.x, -.68 + indent + len);
  vec3 lc = mix(vec3(.42, .41, .45), vColor, step(.62, h1(row + vKind * 3.)));
  col = mix(col, lc, ln * .85);
  float edge = 1. - smoothstep(0., aa * 2.5, abs(d + .012));
  col += vColor * edge * .5;

  // in the ribbon, each frame glows with its category color
  vec3 fill = vColor * (.78 + .38 * vUv.y);
  col = mix(col, fill, vFill * .9);
  col *= mix(1., .4, vDepth * (1. - vFill));

  float a = shape * vAlpha * mix(.78 - vDepth * .35, .92, vFill);

  // stars
  float sd = length(p) / .5;
  float star = exp(-sd * sd * 2.4);
  col = mix(col, mix(vColor, vec3(1., .96, .9), .6) * 1.15, vStar);
  a = mix(a, star * vAlpha * vBright, vStar);

  if (a < .01) discard;
  gl_FragColor = vec4(col, a);
}`;

/* ------------------------------------------------------------------ */
export function createGL(canvas, { mobile = false } = {}) {
  const renderer = new WebGLRenderer({ canvas, antialias: false, alpha: false, powerPreference: 'high-performance' });
  renderer.outputColorSpace = LinearSRGBColorSpace;
  const dpr = Math.min(window.devicePixelRatio || 1, mobile ? 1.5 : 1.75);
  renderer.setPixelRatio(dpr);

  const scene = new Scene();
  const camera = new PerspectiveCamera(35, 1, 0.1, 100);
  camera.position.set(0, 0, 10);

  // Sky
  const skyU = {
    uTop: { value: new Vector3() }, uMid: { value: new Vector3() }, uHor: { value: new Vector3() }, uSun: { value: new Vector3() },
    uSunX: { value: 0.5 }, uSunY: { value: 0 }, uSunI: { value: 1 }, uStars: { value: 0 }, uGround: { value: 1 }, uTime: { value: 0 },
    uRes: { value: new Vector2(1, 1) }, uMouse: { value: new Vector2() },
  };
  const sky = new Mesh(new PlaneGeometry(2, 2), new ShaderMaterial({ vertexShader: skyVert, fragmentShader: skyFrag, uniforms: skyU, depthTest: false, depthWrite: false }));
  sky.frustumCulled = false; sky.renderOrder = -1;
  scene.add(sky);

  // Frames
  const N = mobile ? 800 : 1500;
  const LANES = mobile ? 7 : 10;
  const palette = ['#3ed6b5', '#ffb03b', '#8c7cff', '#ff5b36', '#ff8fb8'].map(hex);
  // a "day" of category segments along the ribbon
  const daySegs = [[0, .08, 3], [.08, .27, 0], [.27, .35, 1], [.35, .48, 2], [.48, .62, 0], [.62, .68, 4], [.68, .75, 3], [.75, .92, 0], [.92, 1, 2]];
  const segColor = (t) => { for (const s of daySegs) if (t >= s[0] && t < s[1]) return s[2]; return 0; };

  const base = new PlaneGeometry(1, 0.625);
  const geo = new InstancedBufferGeometry();
  geo.index = base.index;
  geo.setAttribute('position', base.getAttribute('position'));
  geo.setAttribute('uv', base.getAttribute('uv'));
  const aRand = new Float32Array(N * 4), aScatter = new Float32Array(N * 3), aStar = new Float32Array(N * 3);
  const aT = new Float32Array(N), aLane = new Float32Array(N), aColor = new Float32Array(N * 3), aKind = new Float32Array(N);
  const perLane = Math.ceil(N / LANES);
  for (let i = 0; i < N; i++) {
    const lane = i % LANES, k = Math.floor(i / LANES);
    const t = (k + Math.random() * 0.6) / perLane;
    aT[i] = t;
    aLane[i] = (lane / (LANES - 1)) * 2 - 1;
    const c = palette[Math.random() < 0.12 ? Math.floor(Math.random() * 5) : segColor(t)];
    aColor.set(c, i * 3);
    aRand.set([Math.random(), Math.random(), Math.random(), Math.random()], i * 4);
    // scatter: a deep field, denser around the edges so the headline breathes
    let z, sx, sy, blocked, tries = 0;
    do {
      z = 2.5 - Math.pow(Math.random(), 0.55) * 20;
      const spread = (10 - z) * Math.tan((35 / 2) * Math.PI / 180);
      sx = (Math.random() * 2 - 1) * spread * 1.9; sy = (Math.random() * 2 - 1) * spread * 1.05;
      // keep the headline (lower-left) clear of near frames
      const nx = sx / (spread * 1.6), ny = sy / spread;
      blocked = z > -7 && nx < 0.25 && ny < 0.35 && ny > -0.75;
    } while (blocked && ++tries < 8);
    aScatter.set([sx, sy, z], i * 3);
    const sz = -14 + Math.random() * 10;
    const sp2 = (10 - sz) * Math.tan((35 / 2) * Math.PI / 180);
    aStar.set([(Math.random() * 2 - 1) * sp2 * 1.9, (0.15 + Math.pow(Math.random(), 0.7) * 0.85) * sp2, sz], i * 3);
    aKind[i] = Math.floor(Math.random() * 7);
  }
  geo.setAttribute('aRand', new InstancedBufferAttribute(aRand, 4));
  geo.setAttribute('aScatter', new InstancedBufferAttribute(aScatter, 3));
  geo.setAttribute('aStar', new InstancedBufferAttribute(aStar, 3));
  geo.setAttribute('aT', new InstancedBufferAttribute(aT, 1));
  geo.setAttribute('aLane', new InstancedBufferAttribute(aLane, 1));
  geo.setAttribute('aColor', new InstancedBufferAttribute(aColor, 3));
  geo.setAttribute('aKind', new InstancedBufferAttribute(aKind, 1));
  geo.instanceCount = N;

  const fU = { uTime: { value: 0 }, uMorph: { value: 0 }, uExit: { value: 0 }, uStar: { value: 0 }, uWidth: { value: 14 }, uPulse: { value: 0 }, uMouse: { value: new Vector2(99, 99) } };
  const frames = new Mesh(geo, new ShaderMaterial({ vertexShader: frameVert, fragmentShader: frameFrag, uniforms: fU, transparent: true, depthWrite: false, depthTest: false, side: DoubleSide }));
  frames.frustumCulled = false;
  scene.add(frames);

  // State
  const mouse = { x: 0, y: 0, tx: 0, ty: 0 };
  let halfW = 1, halfH = 1, visible = true;

  function resize() {
    const w = window.innerWidth, h = window.innerHeight;
    renderer.setSize(w, h, false);
    camera.aspect = w / h; camera.updateProjectionMatrix();
    halfH = Math.tan((camera.fov / 2) * Math.PI / 180) * camera.position.z;
    halfW = halfH * camera.aspect;
    fU.uWidth.value = halfW * 2 * 1.25;
    skyU.uRes.value.set(w * dpr, h * dpr);
  }
  resize();
  window.addEventListener('resize', resize);
  window.addEventListener('pointermove', (e) => {
    mouse.tx = (e.clientX / window.innerWidth) * 2 - 1;
    mouse.ty = -((e.clientY / window.innerHeight) * 2 - 1);
  }, { passive: true });
  document.addEventListener('visibilitychange', () => { visible = !document.hidden; });

  const sky0 = SKIES.dawn;
  let skyState = sky0;
  let pulse = 0;

  return {
    u: fU,
    setSky(s) { skyState = s; },
    setHero(morph, exit) { fU.uMorph.value = morph; fU.uExit.value = exit; },
    setStars(v) { fU.uStar.value = v; },
    pulse() { pulse = 1; },
    render(time) {
      if (!visible) return;
      mouse.x += (mouse.tx - mouse.x) * 0.06;
      mouse.y += (mouse.ty - mouse.y) * 0.06;
      pulse *= 0.94;
      const s = skyState;
      skyU.uTop.value.fromArray(s.top); skyU.uMid.value.fromArray(s.mid); skyU.uHor.value.fromArray(s.hor); skyU.uSun.value.fromArray(s.sun);
      skyU.uSunX.value = s.sunX; skyU.uSunY.value = s.sunY; skyU.uSunI.value = s.sunI; skyU.uStars.value = s.stars; skyU.uGround.value = s.ground;
      skyU.uTime.value = time; skyU.uMouse.value.set(mouse.x, mouse.y);
      fU.uTime.value = time; fU.uPulse.value = pulse;
      fU.uMouse.value.set(mouse.x * halfW, mouse.y * halfH);
      camera.position.x = mouse.x * 0.35; camera.position.y = mouse.y * 0.22;
      camera.lookAt(0, 0, 0);
      const showFrames = fU.uExit.value < 1 || fU.uStar.value > 0;
      frames.visible = showFrames;
      renderer.render(scene, camera);
    },
  };
}
