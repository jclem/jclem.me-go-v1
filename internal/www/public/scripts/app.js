window.copyNextCode = (el) => {
  const code = el.closest(".code-example").querySelector("pre").innerText;
  navigator.clipboard.writeText(code);
};
