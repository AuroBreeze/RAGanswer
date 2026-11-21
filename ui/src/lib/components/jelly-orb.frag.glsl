// Jelly Glass Sphere  (Shadertoy image tab)
// Derived from Danil's glass-cube shader (2021+) https://github.com/danilw
// License: Creative Commons Attribution-NonCommercial-ShareAlike 3.0 Unported
//
// Changes vs. the original:
//  - cube  -> sphere (ray-marched SDF), with a soft "jelly" wobble + squash & stretch (volume preserving)
//  - floor / shadow / wallpaper-alpha code removed; the sphere now floats in a gradient void
//  - no specular highlights / outer reflections
//  - inner structure: three layered bilinear-patch sheets, tinted green with multi-colour accents
//  - Y axis is up

// ---------------- tweakables ----------------
#define ELASTICITY     0.8     // 0 = rigid sphere, 1 = jelly, 2 = very bouncy
#define FLOAT_HEIGHT   0.12    // hover amplitude
#define REFRACT_WARP   0.25    // 0 = inner sheets stay straight, 1 = fully bent by the glass
#define AA             2       // 1 = fast, 2 = smooth edges (4 samples), 3+ = slow

#define IOR      1.45
#define PI       3.1415926
#define FDIST    0.7
#define CAM_DIST 4.5
#define SPHERE_R 1.0
#define INNER_R  0.86          // radius of the inner patch structure (in un-squashed space)
#define LIGHT    normalize(vec3(0.5, 0.8, 0.3))

// ---------------- per-frame globals ----------------
float gT;
vec3  gC;   // sphere centre
vec3  gS;   // squash/stretch scale
mat3  gR;   // inner structure rotation
vec3  gU, gV, gW;   // camera right / up / forward

mat3 rotx(float a){float s=sin(a),c=cos(a);return mat3(vec3(1.,0.,0.),vec3(0.,c,s),vec3(0.,-s,c));}
mat3 roty(float a){float s=sin(a),c=cos(a);return mat3(vec3(c,0.,s),vec3(0.,1.,0.),vec3(-s,0.,c));}
mat3 rotz(float a){float s=sin(a),c=cos(a);return mat3(vec3(c,s,0.),vec3(-s,c,0.),vec3(0.,0.,1.));}

float hash12(vec2 p){
    vec3 p3=fract(vec3(p.xyx)*.1031);
    p3+=dot(p3,p3.yzx+33.33);
    return fract((p3.x+p3.y)*p3.z);
}

// ---------------- inner structure: the ORIGINAL three layered bilinear-patch sheets ----------------
vec3 getColor(vec3 p)
{
    p = abs(p);
    p *= 1.25;
    p = 0.5 * p / dot(p, p);
    float t = 0.13 * length(p);

    // soft green-dominant palette: forest -> mint -> teal, with a faint warm accent
    vec3 col = vec3(0.06, 0.18, 0.13);
    float n1 = 0.5 + 0.5 * cos(6.28318 * t * 1.0);
    float n2 = 0.5 + 0.5 * cos(6.28318 * t * 2.7 + 1.6);
    float n3 = 0.5 + 0.5 * cos(6.28318 * t * 5.3 + 3.2);
    float n4 = 0.5 + 0.5 * cos(6.28318 * t * 8.3 + 4.8);

    col = mix(col, vec3(0.42, 0.82, 0.63), n1 * 0.65);
    col = mix(col, vec3(0.16, 0.50, 0.52), n2 * 0.40);
    col = mix(col, vec3(0.60, 0.74, 0.40), n3 * 0.28);
    col += vec3(0.12, 0.07, 0.02) * n4 * 0.35;
    return clamp(col, 0., 1.);
}

void calcColor(vec3 ro, vec3 rd, float d, float len, bool si, float td,
               out vec4 colx, out vec4 colsi)
{
    vec3 pos = ro + rd*d;
    float a = 1. - smoothstep(len - 0.075, len + 0.00001, length(pos));
    colx = vec4(getColor(pos), a);
    colsi = vec4(0.);
    if (si)
    {
        pos = ro + rd*td;
        float ta = 1. - smoothstep(len - 0.075, len + 0.00001, length(pos));
        colsi = vec4(getColor(pos), ta);
    }
}

// bilinear patch intersector (original, iq-based); xSI is self-intersect data, fade fixes dFd on edges
bool iBilinearPatch(in vec3 ro, in vec3 rd, in vec4 ps, in vec4 ph, in float sz,
                    out float t, out vec3 norm, out bool si, out float tsi,
                    out vec3 normsi, out float fade, out float fadesi)
{
    vec3 va = vec3(0.0, 0.0, ph.x + ph.w - ph.y - ph.z);
    vec3 vb = vec3(0.0, ps.w - ps.y, ph.z - ph.x);
    vec3 vc = vec3(ps.z - ps.x, 0.0, ph.y - ph.x);
    vec3 vd = vec3(ps.xy, ph.x);
    t = -1.; tsi = -1.; si = false; fade = 1.; fadesi = 1.;
    norm = vec3(0., 1., 0.); normsi = vec3(0., 1., 0.);

    float tmp = 1.0 / (vb.y * vc.x);
    float a = 0.0, b = 0.0, c = 0.0;
    float d = va.z * tmp;
    float e = 0.0, f = 0.0;
    float g = (vc.z * vb.y - vd.y * va.z) * tmp;
    float h = (vb.z * vc.x - va.z * vd.x) * tmp;
    float i = -1.0;
    float j = (vd.x * vd.y * va.z + vd.z * vb.y * vc.x) * tmp
            - (vd.y * vb.z * vc.x + vd.x * vc.z * vb.y) * tmp;

    float p = dot(vec3(a, b, c), rd.xzy * rd.xzy) + dot(vec3(d, e, f), rd.xzy * rd.zyx);
    float q = dot(vec3(2.0, 2.0, 2.0) * ro.xzy * rd.xyz, vec3(a, b, c))
            + dot(ro.xzz * rd.zxy, vec3(d, d, e))
            + dot(ro.yyx * rd.zxy, vec3(e, f, f))
            + dot(vec3(g, h, i), rd.xzy);
    float r = dot(vec3(a, b, c), ro.xzy * ro.xzy) + dot(vec3(d, e, f), ro.xzy * ro.zyx)
            + dot(vec3(g, h, i), ro.xzy) + j;

    if (abs(p) < 0.000001)
    {
        float tt = -r / q;
        if (tt <= 0.) return false;
        t = tt;
        vec3 pos = ro + t * rd;
        if (length(pos) > sz) return false;
        vec3 grad = vec3(2.0) * pos.xzy * vec3(a, b, c) + pos.zxz * vec3(d, d, e)
                  + pos.yyx * vec3(f, e, f) + vec3(g, h, i);
        norm = -normalize(grad);
        return true;
    }
    else
    {
        float sq = q * q - 4.0 * p * r;
        if (sq < 0.0) return false;
        float s = sqrt(sq);
        float t0 = (-q + s) / (2.0 * p);
        float t1 = (-q - s) / (2.0 * p);
        float tt1 = min(t0 < 0.0 ? t1 : t0, t1 < 0.0 ? t0 : t1);
        float tt2 = max(t0 > 0.0 ? t1 : t0, t1 > 0.0 ? t0 : t1);
        float tt0 = tt1;
        if (tt0 <= 0.) return false;
        vec3 pos = ro + tt0 * rd;
        bool ru = step(sz, length(pos)) > 0.5;
        if (ru)
        {
            tt0 = tt2;
            pos = ro + tt0 * rd;
        }
        if (tt0 <= 0.) return false;
        bool ru2 = step(sz, length(pos)) > 0.5;
        if (ru2) return false;

        // self intersect
        if ((tt2 > 0.) && (!ru) && !(step(sz, length(ro + tt2 * rd)) > 0.5))
        {
            si = true;
            fadesi = s;
            tsi = tt2;
            vec3 tpos = ro + tsi * rd;
            vec3 tgrad = vec3(2.0) * tpos.xzy * vec3(a, b, c) + tpos.zxz * vec3(d, d, e)
                       + tpos.yyx * vec3(f, e, f) + vec3(g, h, i);
            normsi = -normalize(tgrad);
        }

        fade = s;
        t = tt0;
        vec3 grad = vec3(2.0) * pos.xzy * vec3(a, b, c) + pos.zxz * vec3(d, d, e)
                  + pos.yyx * vec3(f, e, f) + vec3(g, h, i);
        norm = -normalize(grad);
        return true;
    }
}

vec4 shadeLayer(vec4 tc, vec3 nrm, vec3 l, float k)
{
    float dif = clamp(dot(nrm, l), 0.0, 1.0);
    float amb = clamp(0.5 + 0.5 * dot(nrm, l), 0.0, 1.0);
    vec3 shad = vec3(0.26, 0.44, 0.34) * amb + vec3(0.85, 1.00, 0.86) * dif;
    const vec3 tcr = vec3(0.16, 0.55, 0.34);
    float ta = clamp(length(tc.rgb) * 1.8, 0., 1.);
    vec3 lit = tc.rgb * shad * k + tcr * tc.rgb * 0.8 * clamp(1. - (amb + dif), 0., 1.);
    return vec4(clamp(lit, 0., 1.), min(tc.a, ta));
}

#define SORT(i,j) if (dx[i].x < dx[j].x){ vec3 tv = dx[i]; dx[i] = dx[j]; dx[j] = tv; \
                                          int ti = order[i]; order[i] = order[j]; order[j] = ti; }

// Three layered, partly transparent saddle sheets inside the sphere.
// Takes a WORLD-space ray, converts it to the squashed/rotated local space.
vec4 insides(vec3 roW, vec3 rdW)
{
    vec3 ro = ((roW - gC) / gS) * gR;
    vec3 rd = (rdW / gS) * gR;
    vec3 l_dir = LIGHT * gR;

    const float curvature = 0.55;
    const float bs = 1.0;
    vec4 ps = vec4(-bs, -bs, bs, bs) * curvature;
    vec4 ph = vec4(-bs, bs, bs, -bs) * curvature;

    vec4 colx[3]   = vec4[3](vec4(0.), vec4(0.), vec4(0.));
    vec3 dx[3]     = vec3[3](vec3(-1.), vec3(-1.), vec3(-1.));
    vec4 colxsi[3] = vec4[3](vec4(0.), vec4(0.), vec4(0.));
    int  order[3]  = int[3](0, 1, 2);

    for (int i = 0; i < 3; i++)
    {
        ro *= rotz(PI/3.);
        rd *= rotz(PI/3.);

        vec3 normnew, normsi;
        float tnew, tsi, fade, fadesi;
        bool si;

        if (iBilinearPatch(ro, rd, ps, ph, INNER_R, tnew, normnew, si, tsi, normsi, fade, fadesi))
        {
            if (tnew > 0.)
            {
                vec4 tcol, tcolsi;
                calcColor(ro, rd, tnew, INNER_R, si, tsi, tcol, tcolsi);
                if (tcol.a > 0.0)
                {
                    dx[i] = vec3(tnew, float(si), tsi);
                    colx[i] = shadeLayer(tcol, normnew, l_dir, 1.2) * min(fade*5., 1.);
                    if (si)
                    {
                        vec4 s = shadeLayer(tcolsi, normsi, l_dir, 0.9);
                        s.rgb *= min(fadesi*5., 1.);
                        colxsi[i] = s;
                    }
                }
            }
        }
    }

    // transparency logic and layer sorting (far -> near)
    float a = 1.;
    SORT(0,1)
    SORT(1,2)
    SORT(0,1)

    if (dx[0].y < 0.5) a = colx[order[0]].a;

    bool rul[3] = bool[3](
        ((dx[0].y > 0.5) && (dx[1].x <= 0.)),
        ((dx[1].y > 0.5) && (dx[0].x > dx[1].z)),
        ((dx[2].y > 0.5) && (dx[1].x > dx[2].z))
    );
    for (int k = 0; k < 3; k++)
    {
        if (rul[k])
        {
            vec4 tcolxsi = colxsi[order[k]];
            vec4 tcolx   = colx[order[k]];
            vec4 tvalx   = mix(tcolxsi, tcolx, tcolx.a);
            colx[order[k]] = mix(vec4(0.), tvalx, max(tcolx.a, tcolxsi.a));
        }
    }

    float a1 = (dx[1].y < 0.5) ? colx[order[1]].a : ((dx[1].z > dx[0].x) ? colx[order[1]].a : 1.);
    float a2 = (dx[2].y < 0.5) ? colx[order[2]].a : ((dx[2].z > dx[1].x) ? colx[order[2]].a : 1.);
    vec3 col = mix(mix(colx[order[0]].rgb, colx[order[1]].rgb, a1), colx[order[2]].rgb, a2);
    a = max(max(a, a1), a2);
    return vec4(col, a);
}

// ---------------- elastic sphere SDF ----------------
float map(vec3 p)
{
    vec3 q = (p - gC) / gS;                 // undo squash & stretch
    float r = length(q);
    vec3 n = q / max(r, 1e-4);
    // low-frequency travelling ripples -> jelly surface
    float w = ELASTICITY * (0.028 * sin(gT*2.0 + n.x*3.0 + n.y*2.0)
                          + 0.022 * sin(gT*2.7 - n.z*3.4 + n.y*1.6)
                          + 0.015 * sin(gT*3.4 + n.x*4.0 - n.z*2.5));
    return (r - (SPHERE_R + w)) * min(gS.x, min(gS.y, gS.z)) * 0.75;
}

vec3 calcN(vec3 p)
{
    const vec2 e = vec2(1., -1.) * 0.0015;
    return normalize(e.xyy*map(p + e.xyy) + e.yyx*map(p + e.yyx) +
                     e.yxy*map(p + e.yxy) + e.xxx*map(p + e.xxx));
}

float march(vec3 ro, vec3 rd)
{
    vec3 oc = ro - gC;
    float b = dot(oc, rd);
    float c = dot(oc, oc) - 1.4*1.4;
    float h = b*b - c;
    if (h < 0.) return -1.;
    h = sqrt(h);
    float t = max(-b - h, 0.);
    float tmax = -b + h;
    for (int i = 0; i < 96; i++)
    {
        float d = map(ro + rd*t);
        if (d < 0.0005) return t;
        t += d;
        if (t > tmax) break;
    }
    return -1.;
}

// distance to the surface when travelling from inside
float exitT(vec3 ro, vec3 rd)
{
    float t = 0.02;
    for (int i = 0; i < 64; i++)
    {
        float d = -map(ro + rd*t);
        if (d < 0.0007) break;
        t += max(d, 0.004);
    }
    return t;
}

// ---------------- environment (no floor) ----------------
vec3 bgcol(vec3 rd)
{
    float k = clamp(1. - pow(abs(rd.y + 0.25), 1.3), 0., 1.);
    return mix(vec3(0.006, 0.016, 0.012), vec3(0.18, 0.40, 0.32), k);
}

// background as seen through the glass (plain gradient, no light boxes)
vec3 env(vec3 rd)
{
    vec3 c = bgcol(rd);
    return c;
}

vec4 render(vec3 ro, vec3 rd)
{
    float t = march(ro, rd);

    if (t < 0.)
    {
        // transparent outside the sphere
        return vec4(0.);
    }

    float R0 = (IOR - 1.) / (IOR + 1.);
    R0 *= R0;

    vec3 p = ro + rd*t;
    vec3 n = calcN(p);

    // refract inside, look at the inner sheets
    vec3 rd2 = refract(rd, n, 1. / IOR);
    float te = exitT(p, rd2);
    vec4 L0 = insides(p, normalize(mix(rd, rd2, REFRACT_WARP)));
    vec3 pe = p + rd2*te;
    vec3 ne = calcN(pe);
    vec3 rdout = refract(rd2, -ne, IOR);
    bool tir = dot(rdout, rdout) < 0.01;
    float fres2 = tir ? 1. : R0 + (1. - R0) * pow(1. - clamp(dot(rdout, ne), 0., 1.), 5.);
    if (tir) rdout = reflect(rd2, -ne);

    vec3 bgOut = env(rdout) * vec3(0.9, 1.0, 0.95);

    // internal reflection -> second pass through the sheets
    vec3 rd3 = reflect(rd2, -ne);
    vec4 L1 = insides(pe, rd3);
    vec3 back = mix(bgOut * 0.35, L1.rgb, L1.a);

    vec3 rest = mix(bgOut, back, fres2);
    vec3 refr = mix(rest, L0.rgb, L0.a);

    vec3 col = refr;      // no outer reflection / specular highlight
    return vec4(clamp(col, 0., 1.), 1.);
}

void mainImage(out vec4 fragColor, in vec2 fragCoord)
{
    gT = iTime;

    // jelly: volume-preserving squash & stretch, lowest when the blob reaches top/bottom of its bob
    float sy  = 1. + ELASTICITY * (0.06 * cos(gT*1.8) + 0.02 * sin(gT*2.6 + 1.3));
    float sxz = inversesqrt(sy);
    gS = vec3(sxz, sy, sxz);
    gC = vec3(0., FLOAT_HEIGHT * sin(gT*0.9), 0.);
    // slow diagonal self-rotation of the inner structure
    gR = roty(gT*0.08) * rotx(0.38) * rotz(gT*0.035);

    // fixed camera angle
    float cy = 0.18;
    float cx = 0.35;
    vec3 eye = CAM_DIST * vec3(cos(cx)*cos(cy), sin(cy), sin(cx)*cos(cy));
    vec3 w = normalize(-eye);
    vec3 u = normalize(cross(w, vec3(0., 1., 0.)));
    vec3 v = cross(u, w);
    gU = u; gV = v; gW = w;

    vec4 tot = vec4(0.);
    for (int mx = 0; mx < AA; mx++)
    for (int nx = 0; nx < AA; nx++)
    {
        vec2 o = (vec2(float(mx), float(nx)) + 0.5) / float(AA) - 0.5;
        vec2 uv = (fragCoord + o - 0.5*iResolution.xy) / iResolution.x;
        vec3 rd = normalize(w*FDIST + uv.x*u + uv.y*v);
        tot += render(eye, rd);
    }
    tot /= float(AA*AA);

    // gentle vignette + dither against banding
    vec2 q = fragCoord / iResolution.xy - 0.5;
    tot.rgb *= 1. - 0.45 * dot(q, q);
    tot.rgb += (hash12(fragCoord + fract(iTime)) - 0.5) / 255. * tot.a;

    fragColor = vec4(clamp(tot.rgb, 0., 1.), clamp(tot.a, 0., 1.));
}
