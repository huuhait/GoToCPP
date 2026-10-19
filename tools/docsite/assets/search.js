// Search over the titles and headings of the pages (assets/search-index.js): no server.
(function () {
  var input = document.getElementById("gp-search");
  var list = document.getElementById("gp-results");
  var root = document.currentScript.getAttribute("data-root") || "";
  if (!input || !window.DOCS_INDEX) return;
  function search(q) {
    var words = q.toLowerCase().split(/\s+/).filter(Boolean);
    if (!words.length) return [];
    var hits = [];
    window.DOCS_INDEX.forEach(function (e) {
      var text = e.t.toLowerCase();
      var ok = words.every(function (w) { return text.indexOf(w) >= 0; });
      if (!ok) return;
      var score = e.l === 0 ? 0 : e.l;
      if (text.indexOf(words.join(" ")) === 0) score -= 2;
      hits.push({ e: e, score: score });
    });
    hits.sort(function (a, b) { return a.score - b.score || a.e.t.length - b.e.t.length; });
    return hits.slice(0, 12).map(function (h) { return h.e; });
  }
  function show() {
    var res = search(input.value);
    list.innerHTML = "";
    res.forEach(function (e) {
      var li = document.createElement("li");
      var a = document.createElement("a");
      a.href = root + e.u;
      a.textContent = e.t;
      if (e.p) { var s = document.createElement("small"); s.textContent = e.p; a.appendChild(s); }
      li.appendChild(a);
      list.appendChild(li);
    });
    list.hidden = res.length === 0;
  }
  input.addEventListener("input", show);
  input.addEventListener("keydown", function (ev) {
    if (ev.key === "Escape") { list.hidden = true; }
    if (ev.key === "Enter") { var a = list.querySelector("a"); if (a) location.href = a.href; }
  });
  document.addEventListener("keydown", function (ev) {
    if (ev.key === "/" && document.activeElement !== input) { ev.preventDefault(); input.focus(); }
  });
})();
