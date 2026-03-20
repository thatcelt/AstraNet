// Compiles a dart2wasm-generated main module from `source` which can then
// instantiatable via the `instantiate` method.
//
// `source` needs to be a `Response` object (or promise thereof) e.g. created
// via the `fetch()` JS API.
export async function compileStreaming(source) {
  const builtins = {builtins: ['js-string']};
  return new CompiledApp(
      await WebAssembly.compileStreaming(source, builtins), builtins);
}

// Compiles a dart2wasm-generated wasm modules from `bytes` which is then
// instantiatable via the `instantiate` method.
export async function compile(bytes) {
  const builtins = {builtins: ['js-string']};
  return new CompiledApp(await WebAssembly.compile(bytes, builtins), builtins);
}

// DEPRECATED: Please use `compile` or `compileStreaming` to get a compiled app,
// use `instantiate` method to get an instantiated app and then call
// `invokeMain` to invoke the main function.
export async function instantiate(modulePromise, importObjectPromise) {
  var moduleOrCompiledApp = await modulePromise;
  if (!(moduleOrCompiledApp instanceof CompiledApp)) {
    moduleOrCompiledApp = new CompiledApp(moduleOrCompiledApp);
  }
  const instantiatedApp = await moduleOrCompiledApp.instantiate(await importObjectPromise);
  return instantiatedApp.instantiatedModule;
}

// DEPRECATED: Please use `compile` or `compileStreaming` to get a compiled app,
// use `instantiate` method to get an instantiated app and then call
// `invokeMain` to invoke the main function.
export const invoke = (moduleInstance, ...args) => {
  moduleInstance.exports.$invokeMain(args);
}

class CompiledApp {
  constructor(module, builtins) {
    this.module = module;
    this.builtins = builtins;
  }

  // The second argument is an options object containing:
  // `loadDeferredWasm` is a JS function that takes a module name matching a
  //   wasm file produced by the dart2wasm compiler and returns the bytes to
  //   load the module. These bytes can be in either a format supported by
  //   `WebAssembly.compile` or `WebAssembly.compileStreaming`.
  // `loadDynamicModule` is a JS function that takes two string names matching,
  //   in order, a wasm file produced by the dart2wasm compiler during dynamic
  //   module compilation and a corresponding js file produced by the same
  //   compilation. It should return a JS Array containing 2 elements. The first
  //   should be the bytes for the wasm module in a format supported by
  //   `WebAssembly.compile` or `WebAssembly.compileStreaming`. The second
  //   should be the result of using the JS 'import' API on the js file path.
  async instantiate(additionalImports, {loadDeferredWasm, loadDynamicModule} = {}) {
    let dartInstance;

    // Prints to the console
    function printToConsole(value) {
      if (typeof dartPrint == "function") {
        dartPrint(value);
        return;
      }
      if (typeof console == "object" && typeof console.log != "undefined") {
        console.log(value);
        return;
      }
      if (typeof print == "function") {
        print(value);
        return;
      }

      throw "Unable to print message: " + value;
    }

    // A special symbol attached to functions that wrap Dart functions.
    const jsWrappedDartFunctionSymbol = Symbol("JSWrappedDartFunction");

    function finalizeWrapper(dartFunction, wrapped) {
      wrapped.dartFunction = dartFunction;
      wrapped[jsWrappedDartFunctionSymbol] = true;
      return wrapped;
    }

    // Imports
    const dart2wasm = {
            _3: (o, t) => typeof o === t,
      _4: (o, c) => o instanceof c,
      _5: o => Object.keys(o),
      _8: (o, a) => o + a,
      _35: () => new Array(),
      _36: x0 => new Array(x0),
      _38: x0 => x0.length,
      _40: (x0,x1) => x0[x1],
      _41: (x0,x1,x2) => { x0[x1] = x2 },
      _43: x0 => new Promise(x0),
      _45: (x0,x1,x2) => new DataView(x0,x1,x2),
      _47: x0 => new Int8Array(x0),
      _48: (x0,x1,x2) => new Uint8Array(x0,x1,x2),
      _49: x0 => new Uint8Array(x0),
      _51: x0 => new Uint8ClampedArray(x0),
      _53: x0 => new Int16Array(x0),
      _55: x0 => new Uint16Array(x0),
      _57: x0 => new Int32Array(x0),
      _59: x0 => new Uint32Array(x0),
      _61: x0 => new Float32Array(x0),
      _63: x0 => new Float64Array(x0),
      _65: (x0,x1,x2) => x0.call(x1,x2),
      _70: (decoder, codeUnits) => decoder.decode(codeUnits),
      _71: () => new TextDecoder("utf-8", {fatal: true}),
      _72: () => new TextDecoder("utf-8", {fatal: false}),
      _73: (s) => +s,
      _74: x0 => new Uint8Array(x0),
      _75: (x0,x1,x2) => x0.set(x1,x2),
      _76: (x0,x1) => x0.transferFromImageBitmap(x1),
      _78: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._78(f,arguments.length,x0) }),
      _79: x0 => new window.FinalizationRegistry(x0),
      _80: (x0,x1,x2,x3) => x0.register(x1,x2,x3),
      _81: (x0,x1) => x0.unregister(x1),
      _82: (x0,x1,x2) => x0.slice(x1,x2),
      _83: (x0,x1) => x0.decode(x1),
      _84: (x0,x1) => x0.segment(x1),
      _85: () => new TextDecoder(),
      _86: (x0,x1) => x0.get(x1),
      _87: x0 => x0.buffer,
      _88: x0 => x0.wasmMemory,
      _89: () => globalThis.window._flutter_skwasmInstance,
      _90: x0 => x0.rasterStartMilliseconds,
      _91: x0 => x0.rasterEndMilliseconds,
      _92: x0 => x0.imageBitmaps,
      _196: x0 => x0.stopPropagation(),
      _197: x0 => x0.preventDefault(),
      _199: x0 => x0.remove(),
      _200: (x0,x1) => x0.append(x1),
      _201: (x0,x1,x2,x3) => x0.addEventListener(x1,x2,x3),
      _246: x0 => x0.unlock(),
      _247: x0 => x0.getReader(),
      _248: (x0,x1,x2) => x0.addEventListener(x1,x2),
      _249: (x0,x1,x2) => x0.removeEventListener(x1,x2),
      _250: (x0,x1) => x0.item(x1),
      _251: x0 => x0.next(),
      _252: x0 => x0.now(),
      _253: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._253(f,arguments.length,x0) }),
      _254: (x0,x1) => x0.addListener(x1),
      _255: (x0,x1) => x0.removeListener(x1),
      _256: (x0,x1) => x0.matchMedia(x1),
      _257: (x0,x1) => x0.revokeObjectURL(x1),
      _258: x0 => x0.close(),
      _259: (x0,x1,x2,x3,x4) => ({type: x0,data: x1,premultiplyAlpha: x2,colorSpaceConversion: x3,preferAnimation: x4}),
      _260: x0 => new window.ImageDecoder(x0),
      _261: x0 => ({frameIndex: x0}),
      _262: (x0,x1) => x0.decode(x1),
      _263: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._263(f,arguments.length,x0) }),
      _264: (x0,x1) => x0.getModifierState(x1),
      _265: (x0,x1) => x0.removeProperty(x1),
      _266: (x0,x1) => x0.prepend(x1),
      _267: x0 => new Intl.Locale(x0),
      _268: x0 => x0.disconnect(),
      _269: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._269(f,arguments.length,x0) }),
      _270: (x0,x1) => x0.getAttribute(x1),
      _271: (x0,x1) => x0.contains(x1),
      _272: (x0,x1) => x0.querySelector(x1),
      _273: x0 => x0.blur(),
      _274: x0 => x0.hasFocus(),
      _275: (x0,x1,x2) => x0.insertBefore(x1,x2),
      _276: (x0,x1) => x0.hasAttribute(x1),
      _277: (x0,x1) => x0.getModifierState(x1),
      _278: (x0,x1) => x0.createTextNode(x1),
      _279: (x0,x1) => x0.appendChild(x1),
      _280: (x0,x1) => x0.removeAttribute(x1),
      _281: x0 => x0.getBoundingClientRect(),
      _282: (x0,x1) => x0.observe(x1),
      _283: x0 => x0.disconnect(),
      _284: (x0,x1) => x0.closest(x1),
      _707: () => globalThis.window.flutterConfiguration,
      _709: x0 => x0.assetBase,
      _714: x0 => x0.canvasKitMaximumSurfaces,
      _715: x0 => x0.debugShowSemanticsNodes,
      _716: x0 => x0.hostElement,
      _717: x0 => x0.multiViewEnabled,
      _718: x0 => x0.nonce,
      _720: x0 => x0.fontFallbackBaseUrl,
      _730: x0 => x0.console,
      _731: x0 => x0.devicePixelRatio,
      _732: x0 => x0.document,
      _733: x0 => x0.history,
      _734: x0 => x0.innerHeight,
      _735: x0 => x0.innerWidth,
      _736: x0 => x0.location,
      _737: x0 => x0.navigator,
      _738: x0 => x0.visualViewport,
      _739: x0 => x0.performance,
      _741: x0 => x0.URL,
      _743: (x0,x1) => x0.getComputedStyle(x1),
      _744: x0 => x0.screen,
      _745: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._745(f,arguments.length,x0) }),
      _746: (x0,x1) => x0.requestAnimationFrame(x1),
      _751: (x0,x1) => x0.warn(x1),
      _753: (x0,x1) => x0.debug(x1),
      _754: x0 => globalThis.parseFloat(x0),
      _755: () => globalThis.window,
      _756: () => globalThis.Intl,
      _757: () => globalThis.Symbol,
      _758: (x0,x1,x2,x3,x4) => globalThis.createImageBitmap(x0,x1,x2,x3,x4),
      _760: x0 => x0.clipboard,
      _761: x0 => x0.maxTouchPoints,
      _762: x0 => x0.vendor,
      _763: x0 => x0.language,
      _764: x0 => x0.platform,
      _765: x0 => x0.userAgent,
      _766: (x0,x1) => x0.vibrate(x1),
      _767: x0 => x0.languages,
      _768: x0 => x0.documentElement,
      _769: (x0,x1) => x0.querySelector(x1),
      _772: (x0,x1) => x0.createElement(x1),
      _775: (x0,x1) => x0.createEvent(x1),
      _776: x0 => x0.activeElement,
      _779: x0 => x0.head,
      _780: x0 => x0.body,
      _782: (x0,x1) => { x0.title = x1 },
      _785: x0 => x0.visibilityState,
      _786: () => globalThis.document,
      _787: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._787(f,arguments.length,x0) }),
      _788: (x0,x1) => x0.dispatchEvent(x1),
      _796: x0 => x0.target,
      _798: x0 => x0.timeStamp,
      _799: x0 => x0.type,
      _801: (x0,x1,x2,x3) => x0.initEvent(x1,x2,x3),
      _807: x0 => x0.baseURI,
      _808: x0 => x0.firstChild,
      _812: x0 => x0.parentElement,
      _814: (x0,x1) => { x0.textContent = x1 },
      _815: x0 => x0.parentNode,
      _816: x0 => x0.nextSibling,
      _817: (x0,x1) => x0.removeChild(x1),
      _818: x0 => x0.isConnected,
      _826: x0 => x0.clientHeight,
      _827: x0 => x0.clientWidth,
      _828: x0 => x0.offsetHeight,
      _829: x0 => x0.offsetWidth,
      _830: x0 => x0.id,
      _831: (x0,x1) => { x0.id = x1 },
      _834: (x0,x1) => { x0.spellcheck = x1 },
      _835: x0 => x0.tagName,
      _836: x0 => x0.style,
      _838: (x0,x1) => x0.querySelectorAll(x1),
      _839: (x0,x1,x2) => x0.setAttribute(x1,x2),
      _840: (x0,x1) => { x0.tabIndex = x1 },
      _841: x0 => x0.tabIndex,
      _842: (x0,x1) => x0.focus(x1),
      _843: x0 => x0.scrollTop,
      _844: (x0,x1) => { x0.scrollTop = x1 },
      _845: x0 => x0.scrollLeft,
      _846: (x0,x1) => { x0.scrollLeft = x1 },
      _847: x0 => x0.classList,
      _849: (x0,x1) => { x0.className = x1 },
      _851: (x0,x1) => x0.getElementsByClassName(x1),
      _852: x0 => x0.click(),
      _853: (x0,x1) => x0.attachShadow(x1),
      _856: x0 => x0.computedStyleMap(),
      _857: (x0,x1) => x0.get(x1),
      _863: (x0,x1) => x0.getPropertyValue(x1),
      _864: (x0,x1,x2,x3) => x0.setProperty(x1,x2,x3),
      _865: x0 => x0.offsetLeft,
      _866: x0 => x0.offsetTop,
      _867: x0 => x0.offsetParent,
      _869: (x0,x1) => { x0.name = x1 },
      _870: x0 => x0.content,
      _871: (x0,x1) => { x0.content = x1 },
      _875: (x0,x1) => { x0.src = x1 },
      _876: x0 => x0.naturalWidth,
      _877: x0 => x0.naturalHeight,
      _881: (x0,x1) => { x0.crossOrigin = x1 },
      _883: (x0,x1) => { x0.decoding = x1 },
      _884: x0 => x0.decode(),
      _889: (x0,x1) => { x0.nonce = x1 },
      _894: (x0,x1) => { x0.width = x1 },
      _896: (x0,x1) => { x0.height = x1 },
      _899: (x0,x1) => x0.getContext(x1),
      _960: x0 => x0.width,
      _961: x0 => x0.height,
      _963: (x0,x1) => x0.fetch(x1),
      _964: x0 => x0.status,
      _965: x0 => x0.headers,
      _966: x0 => x0.body,
      _967: x0 => x0.arrayBuffer(),
      _970: x0 => x0.read(),
      _971: x0 => x0.value,
      _972: x0 => x0.done,
      _979: x0 => x0.name,
      _980: x0 => x0.x,
      _981: x0 => x0.y,
      _984: x0 => x0.top,
      _985: x0 => x0.right,
      _986: x0 => x0.bottom,
      _987: x0 => x0.left,
      _997: x0 => x0.height,
      _998: x0 => x0.width,
      _999: x0 => x0.scale,
      _1000: (x0,x1) => { x0.value = x1 },
      _1003: (x0,x1) => { x0.placeholder = x1 },
      _1005: (x0,x1) => { x0.name = x1 },
      _1006: x0 => x0.selectionDirection,
      _1007: x0 => x0.selectionStart,
      _1008: x0 => x0.selectionEnd,
      _1011: x0 => x0.value,
      _1013: (x0,x1,x2) => x0.setSelectionRange(x1,x2),
      _1014: x0 => x0.readText(),
      _1015: (x0,x1) => x0.writeText(x1),
      _1017: x0 => x0.altKey,
      _1018: x0 => x0.code,
      _1019: x0 => x0.ctrlKey,
      _1020: x0 => x0.key,
      _1021: x0 => x0.keyCode,
      _1022: x0 => x0.location,
      _1023: x0 => x0.metaKey,
      _1024: x0 => x0.repeat,
      _1025: x0 => x0.shiftKey,
      _1026: x0 => x0.isComposing,
      _1028: x0 => x0.state,
      _1029: (x0,x1) => x0.go(x1),
      _1031: (x0,x1,x2,x3) => x0.pushState(x1,x2,x3),
      _1032: (x0,x1,x2,x3) => x0.replaceState(x1,x2,x3),
      _1033: x0 => x0.pathname,
      _1034: x0 => x0.search,
      _1035: x0 => x0.hash,
      _1039: x0 => x0.state,
      _1042: (x0,x1) => x0.createObjectURL(x1),
      _1044: x0 => new Blob(x0),
      _1046: x0 => new MutationObserver(x0),
      _1047: (x0,x1,x2) => x0.observe(x1,x2),
      _1048: f => finalizeWrapper(f, function(x0,x1) { return dartInstance.exports._1048(f,arguments.length,x0,x1) }),
      _1051: x0 => x0.attributeName,
      _1052: x0 => x0.type,
      _1053: x0 => x0.matches,
      _1054: x0 => x0.matches,
      _1058: x0 => x0.relatedTarget,
      _1060: x0 => x0.clientX,
      _1061: x0 => x0.clientY,
      _1062: x0 => x0.offsetX,
      _1063: x0 => x0.offsetY,
      _1066: x0 => x0.button,
      _1067: x0 => x0.buttons,
      _1068: x0 => x0.ctrlKey,
      _1072: x0 => x0.pointerId,
      _1073: x0 => x0.pointerType,
      _1074: x0 => x0.pressure,
      _1075: x0 => x0.tiltX,
      _1076: x0 => x0.tiltY,
      _1077: x0 => x0.getCoalescedEvents(),
      _1080: x0 => x0.deltaX,
      _1081: x0 => x0.deltaY,
      _1082: x0 => x0.wheelDeltaX,
      _1083: x0 => x0.wheelDeltaY,
      _1084: x0 => x0.deltaMode,
      _1091: x0 => x0.changedTouches,
      _1094: x0 => x0.clientX,
      _1095: x0 => x0.clientY,
      _1098: x0 => x0.data,
      _1101: (x0,x1) => { x0.disabled = x1 },
      _1103: (x0,x1) => { x0.type = x1 },
      _1104: (x0,x1) => { x0.max = x1 },
      _1105: (x0,x1) => { x0.min = x1 },
      _1106: x0 => x0.value,
      _1107: (x0,x1) => { x0.value = x1 },
      _1108: x0 => x0.disabled,
      _1109: (x0,x1) => { x0.disabled = x1 },
      _1111: (x0,x1) => { x0.placeholder = x1 },
      _1112: (x0,x1) => { x0.name = x1 },
      _1115: (x0,x1) => { x0.autocomplete = x1 },
      _1116: x0 => x0.selectionDirection,
      _1117: x0 => x0.selectionStart,
      _1119: x0 => x0.selectionEnd,
      _1122: (x0,x1,x2) => x0.setSelectionRange(x1,x2),
      _1123: (x0,x1) => x0.add(x1),
      _1126: (x0,x1) => { x0.noValidate = x1 },
      _1127: (x0,x1) => { x0.method = x1 },
      _1128: (x0,x1) => { x0.action = x1 },
      _1154: x0 => x0.orientation,
      _1155: x0 => x0.width,
      _1156: x0 => x0.height,
      _1157: (x0,x1) => x0.lock(x1),
      _1176: x0 => new ResizeObserver(x0),
      _1179: f => finalizeWrapper(f, function(x0,x1) { return dartInstance.exports._1179(f,arguments.length,x0,x1) }),
      _1187: x0 => x0.length,
      _1188: x0 => x0.iterator,
      _1189: x0 => x0.Segmenter,
      _1190: x0 => x0.v8BreakIterator,
      _1191: (x0,x1) => new Intl.Segmenter(x0,x1),
      _1194: x0 => x0.language,
      _1195: x0 => x0.script,
      _1196: x0 => x0.region,
      _1214: x0 => x0.done,
      _1215: x0 => x0.value,
      _1216: x0 => x0.index,
      _1220: (x0,x1) => new Intl.v8BreakIterator(x0,x1),
      _1221: (x0,x1) => x0.adoptText(x1),
      _1222: x0 => x0.first(),
      _1223: x0 => x0.next(),
      _1224: x0 => x0.current(),
      _1238: x0 => x0.hostElement,
      _1239: x0 => x0.viewConstraints,
      _1242: x0 => x0.maxHeight,
      _1243: x0 => x0.maxWidth,
      _1244: x0 => x0.minHeight,
      _1245: x0 => x0.minWidth,
      _1246: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._1246(f,arguments.length,x0) }),
      _1247: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._1247(f,arguments.length,x0) }),
      _1248: (x0,x1) => ({addView: x0,removeView: x1}),
      _1251: x0 => x0.loader,
      _1252: () => globalThis._flutter,
      _1253: (x0,x1) => x0.didCreateEngineInitializer(x1),
      _1254: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._1254(f,arguments.length,x0) }),
      _1255: f => finalizeWrapper(f, function() { return dartInstance.exports._1255(f,arguments.length) }),
      _1256: (x0,x1) => ({initializeEngine: x0,autoStart: x1}),
      _1259: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._1259(f,arguments.length,x0) }),
      _1260: x0 => ({runApp: x0}),
      _1262: f => finalizeWrapper(f, function(x0,x1) { return dartInstance.exports._1262(f,arguments.length,x0,x1) }),
      _1263: x0 => x0.length,
      _1264: () => globalThis.window.ImageDecoder,
      _1265: x0 => x0.tracks,
      _1267: x0 => x0.completed,
      _1269: x0 => x0.image,
      _1275: x0 => x0.displayWidth,
      _1276: x0 => x0.displayHeight,
      _1277: x0 => x0.duration,
      _1280: x0 => x0.ready,
      _1281: x0 => x0.selectedTrack,
      _1282: x0 => x0.repetitionCount,
      _1283: x0 => x0.frameCount,
      _1326: () => new MediaStream(),
      _1327: x0 => x0.getVideoTracks(),
      _1328: (x0,x1) => x0.addTrack(x1),
      _1329: x0 => x0.getAudioTracks(),
      _1330: (x0,x1) => x0.append(x1),
      _1331: (x0,x1) => x0.getElementById(x1),
      _1334: x0 => x0.remove(),
      _1337: (x0,x1,x2) => x0.setAttribute(x1,x2),
      _1347: x0 => x0.close(),
      _1348: (x0,x1) => x0.send(x1),
      _1349: x0 => new WebSocket(x0),
      _1350: x0 => x0.play(),
      _1357: (x0,x1) => x0.createElement(x1),
      _1363: (x0,x1,x2) => x0.addEventListener(x1,x2),
      _1366: (x0,x1,x2) => x0.addEventListener(x1,x2),
      _1367: (x0,x1,x2,x3) => x0.addEventListener(x1,x2,x3),
      _1368: (x0,x1,x2,x3) => x0.removeEventListener(x1,x2,x3),
      _1369: (x0,x1) => x0.createElement(x1),
      _1374: (x0,x1,x2,x3) => x0.open(x1,x2,x3),
      _1375: () => globalThis.Notification.requestPermission(),
      _1376: x0 => globalThis.URL.revokeObjectURL(x0),
      _1377: (x0,x1,x2,x3) => x0.drawImage(x1,x2,x3),
      _1378: x0 => globalThis.URL.createObjectURL(x0),
      _1379: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._1379(f,arguments.length,x0) }),
      _1380: (x0,x1,x2,x3) => x0.toBlob(x1,x2,x3),
      _1381: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._1381(f,arguments.length,x0) }),
      _1382: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._1382(f,arguments.length,x0) }),
      _1383: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._1383(f,arguments.length,x0) }),
      _1384: (x0,x1) => x0.querySelector(x1),
      _1385: (x0,x1) => x0.replaceChildren(x1),
      _1386: x0 => x0.click(),
      _1387: x0 => x0.decode(),
      _1388: (x0,x1,x2,x3) => x0.open(x1,x2,x3),
      _1389: (x0,x1,x2) => x0.setRequestHeader(x1,x2),
      _1390: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._1390(f,arguments.length,x0) }),
      _1391: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._1391(f,arguments.length,x0) }),
      _1392: x0 => x0.send(),
      _1393: () => new XMLHttpRequest(),
      _1394: (x0,x1) => x0.getItem(x1),
      _1395: (x0,x1) => x0.removeItem(x1),
      _1396: (x0,x1,x2) => x0.setItem(x1,x2),
      _1400: (x0,x1) => x0.getUserMedia(x1),
      _1401: x0 => x0.stop(),
      _1408: (x0,x1) => x0.item(x1),
      _1409: () => new FileReader(),
      _1411: (x0,x1) => x0.readAsArrayBuffer(x1),
      _1419: x0 => x0.deviceMemory,
      _1421: (x0,x1,x2,x3,x4,x5,x6,x7) => ({apiKey: x0,authDomain: x1,databaseURL: x2,projectId: x3,storageBucket: x4,messagingSenderId: x5,measurementId: x6,appId: x7}),
      _1422: (x0,x1) => globalThis.firebase_core.initializeApp(x0,x1),
      _1423: x0 => globalThis.firebase_core.getApp(x0),
      _1424: () => globalThis.firebase_core.getApp(),
      _1426: (x0,x1) => ({next: x0,error: x1}),
      _1427: x0 => ({vapidKey: x0}),
      _1428: x0 => globalThis.firebase_messaging.getMessaging(x0),
      _1430: (x0,x1) => globalThis.firebase_messaging.getToken(x0,x1),
      _1432: (x0,x1) => globalThis.firebase_messaging.onMessage(x0,x1),
      _1436: x0 => x0.title,
      _1437: x0 => x0.body,
      _1438: x0 => x0.image,
      _1439: x0 => x0.messageId,
      _1440: x0 => x0.collapseKey,
      _1441: x0 => x0.fcmOptions,
      _1442: x0 => x0.notification,
      _1443: x0 => x0.data,
      _1444: x0 => x0.from,
      _1445: x0 => x0.analyticsLabel,
      _1446: x0 => x0.link,
      _1447: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._1447(f,arguments.length,x0) }),
      _1448: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._1448(f,arguments.length,x0) }),
      _1450: () => globalThis.firebase_core.SDK_VERSION,
      _1456: x0 => x0.apiKey,
      _1458: x0 => x0.authDomain,
      _1460: x0 => x0.databaseURL,
      _1462: x0 => x0.projectId,
      _1464: x0 => x0.storageBucket,
      _1466: x0 => x0.messagingSenderId,
      _1468: x0 => x0.measurementId,
      _1470: x0 => x0.appId,
      _1472: x0 => x0.name,
      _1473: x0 => x0.options,
      _1474: (x0,x1) => x0.debug(x1),
      _1475: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._1475(f,arguments.length,x0) }),
      _1476: f => finalizeWrapper(f, function(x0,x1) { return dartInstance.exports._1476(f,arguments.length,x0,x1) }),
      _1477: (x0,x1) => ({createScript: x0,createScriptURL: x1}),
      _1478: (x0,x1,x2) => x0.createPolicy(x1,x2),
      _1479: (x0,x1) => x0.createScriptURL(x1),
      _1480: (x0,x1,x2) => x0.createScript(x1,x2),
      _1481: (x0,x1) => x0.appendChild(x1),
      _1482: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._1482(f,arguments.length,x0) }),
      _1484: Date.now,
      _1486: s => new Date(s * 1000).getTimezoneOffset() * 60,
      _1487: s => {
        if (!/^\s*[+-]?(?:Infinity|NaN|(?:\.\d+|\d+(?:\.\d*)?)(?:[eE][+-]?\d+)?)\s*$/.test(s)) {
          return NaN;
        }
        return parseFloat(s);
      },
      _1488: () => {
        let stackString = new Error().stack.toString();
        let frames = stackString.split('\n');
        let drop = 2;
        if (frames[0] === 'Error') {
            drop += 1;
        }
        return frames.slice(drop).join('\n');
      },
      _1489: () => typeof dartUseDateNowForTicks !== "undefined",
      _1490: () => 1000 * performance.now(),
      _1491: () => Date.now(),
      _1492: () => {
        // On browsers return `globalThis.location.href`
        if (globalThis.location != null) {
          return globalThis.location.href;
        }
        return null;
      },
      _1493: () => {
        return typeof process != "undefined" &&
               Object.prototype.toString.call(process) == "[object process]" &&
               process.platform == "win32"
      },
      _1494: () => new WeakMap(),
      _1495: (map, o) => map.get(o),
      _1496: (map, o, v) => map.set(o, v),
      _1497: x0 => new WeakRef(x0),
      _1498: x0 => x0.deref(),
      _1505: () => globalThis.WeakRef,
      _1509: s => JSON.stringify(s),
      _1510: s => printToConsole(s),
      _1511: (o, p, r) => o.replaceAll(p, () => r),
      _1512: (o, p, r) => o.replace(p, () => r),
      _1513: Function.prototype.call.bind(String.prototype.toLowerCase),
      _1514: s => s.toUpperCase(),
      _1515: s => s.trim(),
      _1516: s => s.trimLeft(),
      _1517: s => s.trimRight(),
      _1518: (string, times) => string.repeat(times),
      _1519: Function.prototype.call.bind(String.prototype.indexOf),
      _1520: (s, p, i) => s.lastIndexOf(p, i),
      _1521: (string, token) => string.split(token),
      _1522: Object.is,
      _1523: o => o instanceof Array,
      _1524: (a, i) => a.push(i),
      _1527: (a, l) => a.length = l,
      _1528: a => a.pop(),
      _1529: (a, i) => a.splice(i, 1),
      _1530: (a, s) => a.join(s),
      _1531: (a, s, e) => a.slice(s, e),
      _1533: (a, b) => a == b ? 0 : (a > b ? 1 : -1),
      _1534: a => a.length,
      _1535: (a, l) => a.length = l,
      _1536: (a, i) => a[i],
      _1537: (a, i, v) => a[i] = v,
      _1539: o => {
        if (o instanceof ArrayBuffer) return 0;
        if (globalThis.SharedArrayBuffer !== undefined &&
            o instanceof SharedArrayBuffer) {
          return 1;
        }
        return 2;
      },
      _1540: (o, offsetInBytes, lengthInBytes) => {
        var dst = new ArrayBuffer(lengthInBytes);
        new Uint8Array(dst).set(new Uint8Array(o, offsetInBytes, lengthInBytes));
        return new DataView(dst);
      },
      _1542: o => o instanceof Uint8Array,
      _1543: (o, start, length) => new Uint8Array(o.buffer, o.byteOffset + start, length),
      _1544: o => o instanceof Int8Array,
      _1545: (o, start, length) => new Int8Array(o.buffer, o.byteOffset + start, length),
      _1546: o => o instanceof Uint8ClampedArray,
      _1547: (o, start, length) => new Uint8ClampedArray(o.buffer, o.byteOffset + start, length),
      _1548: o => o instanceof Uint16Array,
      _1549: (o, start, length) => new Uint16Array(o.buffer, o.byteOffset + start, length),
      _1550: o => o instanceof Int16Array,
      _1551: (o, start, length) => new Int16Array(o.buffer, o.byteOffset + start, length),
      _1552: o => o instanceof Uint32Array,
      _1553: (o, start, length) => new Uint32Array(o.buffer, o.byteOffset + start, length),
      _1554: o => o instanceof Int32Array,
      _1555: (o, start, length) => new Int32Array(o.buffer, o.byteOffset + start, length),
      _1557: (o, start, length) => new BigInt64Array(o.buffer, o.byteOffset + start, length),
      _1558: o => o instanceof Float32Array,
      _1559: (o, start, length) => new Float32Array(o.buffer, o.byteOffset + start, length),
      _1560: o => o instanceof Float64Array,
      _1561: (o, start, length) => new Float64Array(o.buffer, o.byteOffset + start, length),
      _1562: (t, s) => t.set(s),
      _1563: l => new DataView(new ArrayBuffer(l)),
      _1564: (o) => new DataView(o.buffer, o.byteOffset, o.byteLength),
      _1565: o => o.byteLength,
      _1566: o => o.buffer,
      _1567: o => o.byteOffset,
      _1568: Function.prototype.call.bind(Object.getOwnPropertyDescriptor(DataView.prototype, 'byteLength').get),
      _1569: (b, o) => new DataView(b, o),
      _1570: (b, o, l) => new DataView(b, o, l),
      _1571: Function.prototype.call.bind(DataView.prototype.getUint8),
      _1572: Function.prototype.call.bind(DataView.prototype.setUint8),
      _1573: Function.prototype.call.bind(DataView.prototype.getInt8),
      _1574: Function.prototype.call.bind(DataView.prototype.setInt8),
      _1575: Function.prototype.call.bind(DataView.prototype.getUint16),
      _1576: Function.prototype.call.bind(DataView.prototype.setUint16),
      _1577: Function.prototype.call.bind(DataView.prototype.getInt16),
      _1578: Function.prototype.call.bind(DataView.prototype.setInt16),
      _1579: Function.prototype.call.bind(DataView.prototype.getUint32),
      _1580: Function.prototype.call.bind(DataView.prototype.setUint32),
      _1581: Function.prototype.call.bind(DataView.prototype.getInt32),
      _1582: Function.prototype.call.bind(DataView.prototype.setInt32),
      _1585: Function.prototype.call.bind(DataView.prototype.getBigInt64),
      _1586: Function.prototype.call.bind(DataView.prototype.setBigInt64),
      _1587: Function.prototype.call.bind(DataView.prototype.getFloat32),
      _1588: Function.prototype.call.bind(DataView.prototype.setFloat32),
      _1589: Function.prototype.call.bind(DataView.prototype.getFloat64),
      _1590: Function.prototype.call.bind(DataView.prototype.setFloat64),
      _1603: (ms, c) =>
      setTimeout(() => dartInstance.exports.$invokeCallback(c),ms),
      _1604: (handle) => clearTimeout(handle),
      _1605: (ms, c) =>
      setInterval(() => dartInstance.exports.$invokeCallback(c), ms),
      _1606: (handle) => clearInterval(handle),
      _1607: (c) =>
      queueMicrotask(() => dartInstance.exports.$invokeCallback(c)),
      _1608: () => Date.now(),
      _1609: (s, m) => {
        try {
          return new RegExp(s, m);
        } catch (e) {
          return String(e);
        }
      },
      _1610: (x0,x1) => x0.exec(x1),
      _1611: (x0,x1) => x0.test(x1),
      _1612: x0 => x0.pop(),
      _1614: o => o === undefined,
      _1616: o => typeof o === 'function' && o[jsWrappedDartFunctionSymbol] === true,
      _1618: o => {
        const proto = Object.getPrototypeOf(o);
        return proto === Object.prototype || proto === null;
      },
      _1619: o => o instanceof RegExp,
      _1620: (l, r) => l === r,
      _1621: o => o,
      _1622: o => o,
      _1623: o => o,
      _1624: b => !!b,
      _1625: o => o.length,
      _1627: (o, i) => o[i],
      _1628: f => f.dartFunction,
      _1629: () => ({}),
      _1630: () => [],
      _1632: () => globalThis,
      _1633: (constructor, args) => {
        const factoryFunction = constructor.bind.apply(
            constructor, [null, ...args]);
        return new factoryFunction();
      },
      _1634: (o, p) => p in o,
      _1635: (o, p) => o[p],
      _1636: (o, p, v) => o[p] = v,
      _1637: (o, m, a) => o[m].apply(o, a),
      _1639: o => String(o),
      _1640: (p, s, f) => p.then(s, (e) => f(e, e === undefined)),
      _1641: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._1641(f,arguments.length,x0) }),
      _1642: f => finalizeWrapper(f, function(x0,x1) { return dartInstance.exports._1642(f,arguments.length,x0,x1) }),
      _1643: o => {
        if (o === undefined) return 1;
        var type = typeof o;
        if (type === 'boolean') return 2;
        if (type === 'number') return 3;
        if (type === 'string') return 4;
        if (o instanceof Array) return 5;
        if (ArrayBuffer.isView(o)) {
          if (o instanceof Int8Array) return 6;
          if (o instanceof Uint8Array) return 7;
          if (o instanceof Uint8ClampedArray) return 8;
          if (o instanceof Int16Array) return 9;
          if (o instanceof Uint16Array) return 10;
          if (o instanceof Int32Array) return 11;
          if (o instanceof Uint32Array) return 12;
          if (o instanceof Float32Array) return 13;
          if (o instanceof Float64Array) return 14;
          if (o instanceof DataView) return 15;
        }
        if (o instanceof ArrayBuffer) return 16;
        // Feature check for `SharedArrayBuffer` before doing a type-check.
        if (globalThis.SharedArrayBuffer !== undefined &&
            o instanceof SharedArrayBuffer) {
            return 17;
        }
        if (o instanceof Promise) return 18;
        return 19;
      },
      _1644: o => [o],
      _1645: (o0, o1) => [o0, o1],
      _1646: (o0, o1, o2) => [o0, o1, o2],
      _1647: (o0, o1, o2, o3) => [o0, o1, o2, o3],
      _1648: (jsArray, jsArrayOffset, wasmArray, wasmArrayOffset, length) => {
        const getValue = dartInstance.exports.$wasmI8ArrayGet;
        for (let i = 0; i < length; i++) {
          jsArray[jsArrayOffset + i] = getValue(wasmArray, wasmArrayOffset + i);
        }
      },
      _1649: (jsArray, jsArrayOffset, wasmArray, wasmArrayOffset, length) => {
        const setValue = dartInstance.exports.$wasmI8ArraySet;
        for (let i = 0; i < length; i++) {
          setValue(wasmArray, wasmArrayOffset + i, jsArray[jsArrayOffset + i]);
        }
      },
      _1650: (jsArray, jsArrayOffset, wasmArray, wasmArrayOffset, length) => {
        const getValue = dartInstance.exports.$wasmI16ArrayGet;
        for (let i = 0; i < length; i++) {
          jsArray[jsArrayOffset + i] = getValue(wasmArray, wasmArrayOffset + i);
        }
      },
      _1651: (jsArray, jsArrayOffset, wasmArray, wasmArrayOffset, length) => {
        const setValue = dartInstance.exports.$wasmI16ArraySet;
        for (let i = 0; i < length; i++) {
          setValue(wasmArray, wasmArrayOffset + i, jsArray[jsArrayOffset + i]);
        }
      },
      _1652: (jsArray, jsArrayOffset, wasmArray, wasmArrayOffset, length) => {
        const getValue = dartInstance.exports.$wasmI32ArrayGet;
        for (let i = 0; i < length; i++) {
          jsArray[jsArrayOffset + i] = getValue(wasmArray, wasmArrayOffset + i);
        }
      },
      _1653: (jsArray, jsArrayOffset, wasmArray, wasmArrayOffset, length) => {
        const setValue = dartInstance.exports.$wasmI32ArraySet;
        for (let i = 0; i < length; i++) {
          setValue(wasmArray, wasmArrayOffset + i, jsArray[jsArrayOffset + i]);
        }
      },
      _1654: (jsArray, jsArrayOffset, wasmArray, wasmArrayOffset, length) => {
        const getValue = dartInstance.exports.$wasmF32ArrayGet;
        for (let i = 0; i < length; i++) {
          jsArray[jsArrayOffset + i] = getValue(wasmArray, wasmArrayOffset + i);
        }
      },
      _1655: (jsArray, jsArrayOffset, wasmArray, wasmArrayOffset, length) => {
        const setValue = dartInstance.exports.$wasmF32ArraySet;
        for (let i = 0; i < length; i++) {
          setValue(wasmArray, wasmArrayOffset + i, jsArray[jsArrayOffset + i]);
        }
      },
      _1656: (jsArray, jsArrayOffset, wasmArray, wasmArrayOffset, length) => {
        const getValue = dartInstance.exports.$wasmF64ArrayGet;
        for (let i = 0; i < length; i++) {
          jsArray[jsArrayOffset + i] = getValue(wasmArray, wasmArrayOffset + i);
        }
      },
      _1657: (jsArray, jsArrayOffset, wasmArray, wasmArrayOffset, length) => {
        const setValue = dartInstance.exports.$wasmF64ArraySet;
        for (let i = 0; i < length; i++) {
          setValue(wasmArray, wasmArrayOffset + i, jsArray[jsArrayOffset + i]);
        }
      },
      _1658: x0 => new ArrayBuffer(x0),
      _1659: s => {
        if (/[[\]{}()*+?.\\^$|]/.test(s)) {
            s = s.replace(/[[\]{}()*+?.\\^$|]/g, '\\$&');
        }
        return s;
      },
      _1660: x0 => x0.input,
      _1661: x0 => x0.index,
      _1662: x0 => x0.groups,
      _1663: x0 => x0.flags,
      _1664: x0 => x0.multiline,
      _1665: x0 => x0.ignoreCase,
      _1666: x0 => x0.unicode,
      _1667: x0 => x0.dotAll,
      _1668: (x0,x1) => { x0.lastIndex = x1 },
      _1669: (o, p) => p in o,
      _1670: (o, p) => o[p],
      _1671: (o, p, v) => o[p] = v,
      _1672: (o, p) => delete o[p],
      _1673: x0 => globalThis.Object.keys(x0),
      _1675: x0 => new Date(x0),
      _1677: x0 => x0.getTime(),
      _1678: x0 => x0.length,
      _1679: x0 => x0.message,
      _1680: x0 => x0.name,
      _1712: (x0,x1) => x0.add(x1),
      _1732: (x0,x1) => ({keyPath: x0,autoIncrement: x1}),
      _1733: (x0,x1,x2) => x0.createObjectStore(x1,x2),
      _1735: x0 => x0.close(),
      _1738: (x0,x1,x2) => x0.open(x1,x2),
      _1752: (x0,x1) => x0.item(x1),
      _1753: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._1753(f,arguments.length,x0) }),
      _1754: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._1754(f,arguments.length,x0) }),
      _1755: (x0,x1) => x0.replaceTrack(x1),
      _1756: x0 => x0.getParameters(),
      _1757: (x0,x1) => x0.setParameters(x1),
      _1758: x0 => x0.getStats(),
      _1759: f => finalizeWrapper(f, function(x0,x1) { return dartInstance.exports._1759(f,arguments.length,x0,x1) }),
      _1761: (x0,x1) => x0.setCodecPreferences(x1),
      _1762: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._1762(f,arguments.length,x0) }),
      _1763: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._1763(f,arguments.length,x0) }),
      _1764: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._1764(f,arguments.length,x0) }),
      _1765: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._1765(f,arguments.length,x0) }),
      _1766: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._1766(f,arguments.length,x0) }),
      _1767: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._1767(f,arguments.length,x0) }),
      _1768: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._1768(f,arguments.length,x0) }),
      _1769: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._1769(f,arguments.length,x0) }),
      _1770: x0 => x0.close(),
      _1771: (x0,x1) => x0.setConfiguration(x1),
      _1772: (x0,x1) => x0.createOffer(x1),
      _1773: (x0,x1) => x0.createAnswer(x1),
      _1776: (x0,x1) => ({type: x0,sdp: x1}),
      _1777: (x0,x1) => x0.setLocalDescription(x1),
      _1778: (x0,x1) => ({type: x0,sdp: x1}),
      _1779: (x0,x1) => x0.setRemoteDescription(x1),
      _1780: (x0,x1,x2) => ({candidate: x0,sdpMid: x1,sdpMLineIndex: x2}),
      _1781: (x0,x1) => x0.addIceCandidate(x1),
      _1783: x0 => x0.getStats(),
      _1784: f => finalizeWrapper(f, function(x0,x1) { return dartInstance.exports._1784(f,arguments.length,x0,x1) }),
      _1787: (x0,x1,x2,x3) => ({ordered: x0,protocol: x1,negotiated: x2,id: x3}),
      _1788: (x0,x1,x2) => x0.createDataChannel(x1,x2),
      _1789: x0 => x0.restartIce(),
      _1792: (x0,x1) => x0.removeTrack(x1),
      _1793: x0 => x0.getSenders(),
      _1796: (x0,x1,x2) => x0.addTransceiver(x1,x2),
      _1798: (x0,x1) => { x0.binaryType = x1 },
      _1800: x0 => globalThis.RTCRtpReceiver.getCapabilities(x0),
      _1801: x0 => new RTCPeerConnection(x0),
      _1810: x0 => x0.getStats(),
      _1811: f => finalizeWrapper(f, function(x0,x1) { return dartInstance.exports._1811(f,arguments.length,x0,x1) }),
      _1813: (x0,x1) => ({video: x0,audio: x1}),
      _1814: (x0,x1) => x0.getDisplayMedia(x1),
      _1815: x0 => x0.enumerateDevices(),
      _1817: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._1817(f,arguments.length,x0) }),
      _1827: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._1827(f,arguments.length,x0) }),
      _1828: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._1828(f,arguments.length,x0) }),
      _1829: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._1829(f,arguments.length,x0) }),
      _1832: x0 => x0.getSettings(),
      _1835: (x0,x1) => x0.getContext(x1),
      _1839: x0 => x0.arrayBuffer(),
      _1844: () => new XMLHttpRequest(),
      _1845: (x0,x1,x2,x3) => x0.open(x1,x2,x3),
      _1847: (x0,x1,x2) => x0.setRequestHeader(x1,x2),
      _1848: (x0,x1) => x0.send(x1),
      _1849: x0 => x0.send(),
      _1851: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._1851(f,arguments.length,x0) }),
      _1852: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._1852(f,arguments.length,x0) }),
      _1858: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._1858(f,arguments.length,x0) }),
      _1859: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._1859(f,arguments.length,x0) }),
      _1860: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._1860(f,arguments.length,x0) }),
      _1861: f => finalizeWrapper(f, function(x0) { return dartInstance.exports._1861(f,arguments.length,x0) }),
      _1862: (x0,x1) => x0.send(x1),
      _1864: (x0,x1) => new WebSocket(x0,x1),
      _1865: (x0,x1,x2) => x0.close(x1,x2),
      _1867: (x0,x1,x2) => x0.open(x1,x2),
      _1868: x0 => x0.abort(),
      _1869: x0 => x0.getAllResponseHeaders(),
      _1873: () => new AbortController(),
      _1874: x0 => x0.abort(),
      _1875: (x0,x1,x2,x3,x4,x5) => ({method: x0,headers: x1,body: x2,credentials: x3,redirect: x4,signal: x5}),
      _1876: (x0,x1) => globalThis.fetch(x0,x1),
      _1877: (x0,x1) => x0.get(x1),
      _1878: f => finalizeWrapper(f, function(x0,x1,x2) { return dartInstance.exports._1878(f,arguments.length,x0,x1,x2) }),
      _1879: (x0,x1) => x0.forEach(x1),
      _1880: x0 => x0.getReader(),
      _1881: x0 => x0.cancel(),
      _1882: x0 => x0.read(),
      _1883: (x0,x1) => x0.key(x1),
      _1884: x0 => x0.trustedTypes,
      _1885: (x0,x1) => { x0.text = x1 },
      _1886: x0 => x0.random(),
      _1887: (x0,x1) => x0.getRandomValues(x1),
      _1888: () => globalThis.crypto,
      _1889: () => globalThis.Math,
      _1895: (x0,x1,x2,x3,x4,x5) => x0.drawImage(x1,x2,x3,x4,x5),
      _1897: Function.prototype.call.bind(Number.prototype.toString),
      _1898: Function.prototype.call.bind(BigInt.prototype.toString),
      _1899: Function.prototype.call.bind(Number.prototype.toString),
      _1900: (d, digits) => d.toFixed(digits),
      _1904: () => globalThis.document,
      _1910: (x0,x1) => { x0.height = x1 },
      _1912: (x0,x1) => { x0.width = x1 },
      _1921: x0 => x0.style,
      _1924: x0 => x0.src,
      _1925: (x0,x1) => { x0.src = x1 },
      _1926: x0 => x0.naturalWidth,
      _1927: x0 => x0.naturalHeight,
      _1943: x0 => x0.status,
      _1944: (x0,x1) => { x0.responseType = x1 },
      _1946: x0 => x0.response,
      _1984: x0 => x0.readyState,
      _1986: (x0,x1) => { x0.timeout = x1 },
      _1988: (x0,x1) => { x0.withCredentials = x1 },
      _1989: x0 => x0.upload,
      _1990: x0 => x0.responseURL,
      _1991: x0 => x0.status,
      _1992: x0 => x0.statusText,
      _1994: (x0,x1) => { x0.responseType = x1 },
      _1995: x0 => x0.response,
      _2007: x0 => x0.loaded,
      _2008: x0 => x0.total,
      _2031: x0 => x0.length,
      _2070: x0 => x0.style,
      _2083: (x0,x1) => { x0.oncancel = x1 },
      _2089: (x0,x1) => { x0.onchange = x1 },
      _2129: (x0,x1) => { x0.onerror = x1 },
      _2502: (x0,x1) => { x0.src = x1 },
      _2513: x0 => x0.width,
      _2515: x0 => x0.height,
      _2677: (x0,x1) => { x0.srcObject = x1 },
      _2701: (x0,x1) => { x0.autoplay = x1 },
      _2996: (x0,x1) => { x0.accept = x1 },
      _3010: x0 => x0.files,
      _3036: (x0,x1) => { x0.multiple = x1 },
      _3054: (x0,x1) => { x0.type = x1 },
      _3306: (x0,x1) => { x0.type = x1 },
      _3314: (x0,x1) => { x0.crossOrigin = x1 },
      _3316: (x0,x1) => { x0.text = x1 },
      _3348: x0 => x0.width,
      _3349: (x0,x1) => { x0.width = x1 },
      _3350: x0 => x0.height,
      _3351: (x0,x1) => { x0.height = x1 },
      _3771: () => globalThis.window,
      _3811: x0 => x0.document,
      _3833: x0 => x0.navigator,
      _4090: x0 => x0.indexedDB,
      _4095: x0 => x0.trustedTypes,
      _4097: x0 => x0.localStorage,
      _4203: x0 => x0.geolocation,
      _4206: x0 => x0.mediaDevices,
      _4208: x0 => x0.permissions,
      _4209: x0 => x0.maxTouchPoints,
      _4216: x0 => x0.appCodeName,
      _4217: x0 => x0.appName,
      _4218: x0 => x0.appVersion,
      _4219: x0 => x0.platform,
      _4220: x0 => x0.product,
      _4221: x0 => x0.productSub,
      _4222: x0 => x0.userAgent,
      _4223: x0 => x0.vendor,
      _4224: x0 => x0.vendorSub,
      _4226: x0 => x0.language,
      _4227: x0 => x0.languages,
      _4228: x0 => x0.onLine,
      _4233: x0 => x0.hardwareConcurrency,
      _4272: x0 => x0.data,
      _4428: x0 => x0.length,
      _4645: x0 => x0.readyState,
      _4654: x0 => x0.protocol,
      _4658: (x0,x1) => { x0.binaryType = x1 },
      _4661: x0 => x0.code,
      _4662: x0 => x0.reason,
      _4708: x0 => x0.localDescription,
      _4711: x0 => x0.remoteDescription,
      _4714: x0 => x0.signalingState,
      _4715: x0 => x0.iceGatheringState,
      _4716: x0 => x0.iceConnectionState,
      _4717: x0 => x0.connectionState,
      _4730: (x0,x1) => { x0.onicegatheringstatechange = x1 },
      _4742: x0 => x0.type,
      _4743: x0 => x0.sdp,
      _4744: x0 => x0.type,
      _4746: x0 => x0.sdp,
      _4754: x0 => x0.candidate,
      _4755: x0 => x0.sdpMid,
      _4756: x0 => x0.sdpMLineIndex,
      _4778: x0 => x0.candidate,
      _4814: x0 => x0.track,
      _4820: x0 => x0.headerExtensions,
      _4822: x0 => x0.rtcp,
      _4830: (x0,x1) => { x0.encodings = x1 },
      _4838: x0 => x0.active,
      _4853: x0 => x0.cname,
      _4855: x0 => x0.reducedSize,
      _4867: x0 => x0.clockRate,
      _4874: x0 => x0.payloadType,
      _4877: x0 => x0.codecs,
      _4879: x0 => x0.headerExtensions,
      _4904: x0 => x0.sender,
      _4947: x0 => x0.receiver,
      _4948: x0 => x0.track,
      _4949: x0 => x0.streams,
      _4950: x0 => x0.transceiver,
      _4966: x0 => x0.label,
      _4972: x0 => x0.id,
      _4974: x0 => x0.bufferedAmount,
      _4975: x0 => x0.bufferedAmountLowThreshold,
      _4976: (x0,x1) => { x0.bufferedAmountLowThreshold = x1 },
      _4978: (x0,x1) => { x0.onopen = x1 },
      _4980: (x0,x1) => { x0.onbufferedamountlow = x1 },
      _4986: (x0,x1) => { x0.onclose = x1 },
      _4988: (x0,x1) => { x0.onmessage = x1 },
      _4995: (x0,x1) => { x0.maxPacketLifeTime = x1 },
      _4997: (x0,x1) => { x0.maxRetransmits = x1 },
      _5007: x0 => x0.channel,
      _6299: x0 => x0.type,
      _6300: x0 => x0.target,
      _6340: x0 => x0.signal,
      _6394: x0 => x0.baseURI,
      _6411: () => globalThis.document,
      _6492: x0 => x0.body,
      _6494: x0 => x0.head,
      _6823: (x0,x1) => { x0.id = x1 },
      _6850: x0 => x0.children,
      _8169: x0 => x0.value,
      _8171: x0 => x0.done,
      _8350: x0 => x0.size,
      _8351: x0 => x0.type,
      _8358: x0 => x0.name,
      _8359: x0 => x0.lastModified,
      _8364: x0 => x0.length,
      _8369: x0 => x0.result,
      _8588: () => globalThis.Notification.permission,
      _8863: x0 => x0.url,
      _8865: x0 => x0.status,
      _8867: x0 => x0.statusText,
      _8868: x0 => x0.headers,
      _8869: x0 => x0.body,
      _9652: x0 => x0.id,
      _9659: x0 => x0.kind,
      _9660: x0 => x0.id,
      _9661: x0 => x0.label,
      _9662: x0 => x0.enabled,
      _9663: (x0,x1) => { x0.enabled = x1 },
      _9664: x0 => x0.muted,
      _9897: x0 => x0.width,
      _9899: x0 => x0.height,
      _9901: x0 => x0.aspectRatio,
      _9903: x0 => x0.frameRate,
      _9905: x0 => x0.facingMode,
      _9907: x0 => x0.resizeMode,
      _9909: x0 => x0.sampleRate,
      _9911: x0 => x0.sampleSize,
      _9913: x0 => x0.echoCancellation,
      _9915: x0 => x0.autoGainControl,
      _9917: x0 => x0.noiseSuppression,
      _9919: x0 => x0.latency,
      _9921: x0 => x0.channelCount,
      _9923: x0 => x0.deviceId,
      _9925: x0 => x0.groupId,
      _9979: (x0,x1) => { x0.ondevicechange = x1 },
      _9981: x0 => x0.deviceId,
      _9982: x0 => x0.kind,
      _9983: x0 => x0.label,
      _9984: x0 => x0.groupId,
      _10309: x0 => x0.result,
      _10310: x0 => x0.error,
      _10315: (x0,x1) => { x0.onsuccess = x1 },
      _10317: (x0,x1) => { x0.onerror = x1 },
      _10337: x0 => x0.name,
      _10339: x0 => x0.objectStoreNames,
      _10356: x0 => x0.name,
      _10358: x0 => x0.keyPath,
      _10361: x0 => x0.autoIncrement,
      _11222: (x0,x1) => { x0.display = x1 },
      _12444: x0 => x0.name,
      _12445: x0 => x0.message,
      _13160: () => globalThis.console,
      _13188: x0 => x0.name,
      _13189: x0 => x0.message,
      _13190: x0 => x0.code,

    };

    const baseImports = {
      dart2wasm: dart2wasm,
      Math: Math,
      Date: Date,
      Object: Object,
      Array: Array,
      Reflect: Reflect,
      S: new Proxy({}, { get(_, prop) { return prop; } }),

    };

    const jsStringPolyfill = {
      "charCodeAt": (s, i) => s.charCodeAt(i),
      "compare": (s1, s2) => {
        if (s1 < s2) return -1;
        if (s1 > s2) return 1;
        return 0;
      },
      "concat": (s1, s2) => s1 + s2,
      "equals": (s1, s2) => s1 === s2,
      "fromCharCode": (i) => String.fromCharCode(i),
      "length": (s) => s.length,
      "substring": (s, a, b) => s.substring(a, b),
      "fromCharCodeArray": (a, start, end) => {
        if (end <= start) return '';

        const read = dartInstance.exports.$wasmI16ArrayGet;
        let result = '';
        let index = start;
        const chunkLength = Math.min(end - index, 500);
        let array = new Array(chunkLength);
        while (index < end) {
          const newChunkLength = Math.min(end - index, 500);
          for (let i = 0; i < newChunkLength; i++) {
            array[i] = read(a, index++);
          }
          if (newChunkLength < chunkLength) {
            array = array.slice(0, newChunkLength);
          }
          result += String.fromCharCode(...array);
        }
        return result;
      },
      "intoCharCodeArray": (s, a, start) => {
        if (s === '') return 0;

        const write = dartInstance.exports.$wasmI16ArraySet;
        for (var i = 0; i < s.length; ++i) {
          write(a, start++, s.charCodeAt(i));
        }
        return s.length;
      },
      "test": (s) => typeof s == "string",
    };


    

    dartInstance = await WebAssembly.instantiate(this.module, {
      ...baseImports,
      ...additionalImports,
      
      "wasm:js-string": jsStringPolyfill,
    });

    return new InstantiatedApp(this, dartInstance);
  }
}

class InstantiatedApp {
  constructor(compiledApp, instantiatedModule) {
    this.compiledApp = compiledApp;
    this.instantiatedModule = instantiatedModule;
  }

  // Call the main function with the given arguments.
  invokeMain(...args) {
    this.instantiatedModule.exports.$invokeMain(args);
  }
}
