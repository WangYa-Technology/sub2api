import * as THREE from "./three.module.min.js";

function makeCanvasTexture(width, height, paint) {
  const canvas = document.createElement("canvas");
  canvas.width = width;
  canvas.height = height;
  const context = canvas.getContext("2d");
  paint(context, width, height);
  const texture = new THREE.CanvasTexture(canvas);
  texture.colorSpace = THREE.SRGBColorSpace;
  texture.anisotropy = 4;
  return texture;
}

function seededRandom(seed) {
  let value = seed >>> 0;
  return function () {
    value = (value * 1664525 + 1013904223) >>> 0;
    return value / 4294967296;
  };
}

function createSunTexture() {
  const random = seededRandom(56);
  return makeCanvasTexture(768, 384, function (context, width, height) {
    const gradient = context.createLinearGradient(0, 0, width, height);
    gradient.addColorStop(0, "#ffcc55");
    gradient.addColorStop(0.45, "#ff742e");
    gradient.addColorStop(1, "#c72913");
    context.fillStyle = gradient;
    context.fillRect(0, 0, width, height);

    for (let index = 0; index < 170; index += 1) {
      const x = random() * width;
      const y = random() * height;
      const radius = 3 + random() * 15;
      const flare = context.createRadialGradient(x, y, 0, x, y, radius);
      flare.addColorStop(0, "rgba(255,245,170,.78)");
      flare.addColorStop(0.42, "rgba(255,156,58,.34)");
      flare.addColorStop(1, "rgba(129,20,8,0)");
      context.fillStyle = flare;
      context.beginPath();
      context.arc(x, y, radius, 0, Math.PI * 2);
      context.fill();
    }
  });
}

function drawContinent(context, points, scaleX, scaleY) {
  context.beginPath();
  points.forEach(function (point, index) {
    const x = point[0] * scaleX;
    const y = point[1] * scaleY;
    if (index === 0) context.moveTo(x, y);
    else context.lineTo(x, y);
  });
  context.closePath();
  context.fill();
}

function createEarthTexture() {
  return makeCanvasTexture(1024, 512, function (context, width, height) {
    const ocean = context.createLinearGradient(0, 0, 0, height);
    ocean.addColorStop(0, "#143f78");
    ocean.addColorStop(0.5, "#087eaa");
    ocean.addColorStop(1, "#092f69");
    context.fillStyle = ocean;
    context.fillRect(0, 0, width, height);

    context.fillStyle = "#62a85a";
    drawContinent(context, [[.08,.25],[.16,.14],[.24,.19],[.29,.31],[.23,.41],[.19,.56],[.12,.48],[.06,.36]], width, height);
    drawContinent(context, [[.27,.55],[.34,.50],[.39,.58],[.37,.73],[.32,.91],[.27,.78]], width, height);
    drawContinent(context, [[.49,.23],[.57,.15],[.70,.20],[.79,.31],[.76,.42],[.66,.45],[.61,.61],[.54,.55],[.50,.39]], width, height);
    drawContinent(context, [[.57,.55],[.65,.52],[.70,.65],[.66,.84],[.59,.76]], width, height);
    drawContinent(context, [[.81,.61],[.89,.57],[.95,.66],[.91,.79],[.83,.76]], width, height);

    context.fillStyle = "#b9cc76";
    context.globalAlpha = 0.58;
    context.fillRect(0, height * .46, width, height * .09);
    context.globalAlpha = 1;

    const ice = context.createLinearGradient(0, 0, 0, height * .17);
    ice.addColorStop(0, "rgba(245,252,255,.98)");
    ice.addColorStop(1, "rgba(210,240,244,0)");
    context.fillStyle = ice;
    context.fillRect(0, 0, width, height * .2);
    context.save();
    context.translate(0, height);
    context.scale(1, -1);
    context.fillRect(0, 0, width, height * .16);
    context.restore();
  });
}

function createCloudTexture() {
  const random = seededRandom(156);
  return makeCanvasTexture(1024, 512, function (context, width, height) {
    context.clearRect(0, 0, width, height);
    for (let index = 0; index < 55; index += 1) {
      const x = random() * width;
      const y = height * (.12 + random() * .76);
      const rx = 18 + random() * 48;
      const ry = 3 + random() * 9;
      const cloud = context.createRadialGradient(x, y, 0, x, y, rx);
      cloud.addColorStop(0, "rgba(255,255,255,.7)");
      cloud.addColorStop(.55, "rgba(255,255,255,.26)");
      cloud.addColorStop(1, "rgba(255,255,255,0)");
      context.fillStyle = cloud;
      context.beginPath();
      context.ellipse(x, y, rx, ry, random() * .5, 0, Math.PI * 2);
      context.fill();
    }
  });
}

function createMoonTexture() {
  const random = seededRandom(256);
  return makeCanvasTexture(768, 384, function (context, width, height) {
    const base = context.createLinearGradient(0, 0, width, height);
    base.addColorStop(0, "#d7d7d2");
    base.addColorStop(.5, "#8e908f");
    base.addColorStop(1, "#555b60");
    context.fillStyle = base;
    context.fillRect(0, 0, width, height);

    for (let index = 0; index < 95; index += 1) {
      const x = random() * width;
      const y = random() * height;
      const radius = 2 + random() * 13;
      context.fillStyle = "rgba(50,55,58," + (.1 + random() * .23) + ")";
      context.beginPath();
      context.arc(x, y, radius, 0, Math.PI * 2);
      context.fill();
      context.strokeStyle = "rgba(240,240,230,.16)";
      context.lineWidth = Math.max(1, radius * .16);
      context.stroke();
    }
  });
}

function createPlanet(kind) {
  const group = new THREE.Group();
  const geometry = new THREE.SphereGeometry(1, 72, 48);
  let planet;

  if (kind === "sun") {
    planet = new THREE.Mesh(geometry, new THREE.MeshBasicMaterial({ map: createSunTexture() }));
    const glow = new THREE.Mesh(
      new THREE.SphereGeometry(1.14, 48, 32),
      new THREE.MeshBasicMaterial({
        color: 0xff7a2d,
        transparent: true,
        opacity: .18,
        side: THREE.BackSide,
        blending: THREE.AdditiveBlending
      })
    );
    glow.name = "glow";
    group.add(planet, glow);
  } else if (kind === "earth") {
    planet = new THREE.Mesh(
      geometry,
      new THREE.MeshStandardMaterial({ map: createEarthTexture(), roughness: .76, metalness: 0 })
    );
    const clouds = new THREE.Mesh(
      new THREE.SphereGeometry(1.018, 72, 48),
      new THREE.MeshStandardMaterial({
        map: createCloudTexture(),
        transparent: true,
        opacity: .72,
        depthWrite: false,
        roughness: 1
      })
    );
    clouds.name = "clouds";
    const atmosphere = new THREE.Mesh(
      new THREE.SphereGeometry(1.075, 48, 32),
      new THREE.MeshBasicMaterial({
        color: 0x62cfff,
        transparent: true,
        opacity: .11,
        side: THREE.BackSide,
        blending: THREE.AdditiveBlending
      })
    );
    group.add(planet, clouds, atmosphere);
  } else {
    const moonTexture = createMoonTexture();
    planet = new THREE.Mesh(
      geometry,
      new THREE.MeshStandardMaterial({
        map: moonTexture,
        bumpMap: moonTexture,
        bumpScale: .075,
        roughness: 1,
        metalness: 0
      })
    );
    group.add(planet);
  }

  planet.name = "planet";
  group.rotation.z = kind === "earth" ? -.18 : kind === "moon" ? .08 : -.08;
  return group;
}

function mountPlanet(container) {
  if (container.dataset.planetReady === "true") return;
  container.dataset.planetReady = "true";

  const kind = container.dataset.planet;
  const scene = new THREE.Scene();
  const camera = new THREE.PerspectiveCamera(34, 1, .1, 100);
  camera.position.set(0, 0, 4.15);

  const renderer = new THREE.WebGLRenderer({
    alpha: true,
    antialias: true,
    powerPreference: "high-performance",
    preserveDrawingBuffer: true
  });
  renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));
  renderer.setClearColor(0x000000, 0);
  renderer.outputColorSpace = THREE.SRGBColorSpace;
  container.appendChild(renderer.domElement);

  const planetGroup = createPlanet(kind);
  scene.add(planetGroup);
  scene.add(new THREE.AmbientLight(0xffffff, kind === "moon" ? .55 : .8));
  const keyLight = new THREE.DirectionalLight(kind === "moon" ? 0xdde8ff : 0xffffff, kind === "moon" ? 2.8 : 2.1);
  keyLight.position.set(-2.4, 2.2, 3.5);
  scene.add(keyLight);
  const rimLight = new THREE.DirectionalLight(kind === "earth" ? 0x4fc8ff : 0xff9e54, .9);
  rimLight.position.set(3, -1, 1);
  scene.add(rimLight);

  let visible = true;
  let frame = 0;
  let lastTime = 0;
  const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;

  function resize() {
    const rect = container.getBoundingClientRect();
    const width = Math.max(1, Math.round(rect.width));
    const height = Math.max(1, Math.round(rect.height));
    renderer.setSize(width, height, false);
    camera.aspect = width / height;
    camera.updateProjectionMatrix();
  }

  function render(time) {
    if (!document.body.contains(container)) {
      renderer.dispose();
      return;
    }

    const delta = Math.min(40, time - lastTime || 16);
    lastTime = time;
    if (visible && !reducedMotion) {
      const planet = planetGroup.getObjectByName("planet");
      const clouds = planetGroup.getObjectByName("clouds");
      const glow = planetGroup.getObjectByName("glow");
      planet.rotation.y += delta * (kind === "sun" ? .00023 : kind === "earth" ? .00016 : .000085);
      container.dataset.planetRotation = planet.rotation.y.toFixed(4);
      if (clouds) clouds.rotation.y += delta * .00022;
      if (glow) {
        const pulse = 1 + Math.sin(time * .0017) * .025;
        glow.scale.setScalar(pulse);
        glow.material.opacity = .16 + Math.sin(time * .0015) * .035;
      }
    }

    renderer.render(scene, camera);
    frame = window.requestAnimationFrame(render);
  }

  const resizeObserver = new ResizeObserver(resize);
  resizeObserver.observe(container);
  const visibilityObserver = new IntersectionObserver(function (entries) {
    visible = entries[0]?.isIntersecting !== false;
  }, { rootMargin: "120px" });
  visibilityObserver.observe(container);

  resize();
  window.cancelAnimationFrame(frame);
  frame = window.requestAnimationFrame(render);
}

export function initPlanets(root = document) {
  root.querySelectorAll("[data-planet]").forEach(mountPlanet);
}
