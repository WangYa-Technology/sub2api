(function () {
  if (window.location.pathname === "/hcai/page.html") {
    window.history.replaceState(null, "", "/home" + window.location.search + window.location.hash);
  }

  const body = document.body;
  const menuToggle = document.querySelector("[data-menu-toggle]");
  const menuPanel = document.querySelector("[data-menu-panel]");
  const header = document.querySelector(".hc-header");
  const notice = document.querySelector("[data-notice]");
  const closeNotice = document.querySelector("[data-close-notice]");
  const faqItems = Array.from(document.querySelectorAll(".hc-faq__item"));
  const routerModels = Array.from(document.querySelectorAll(".hc-frontier-model[data-model-name]"));
  const routerModelLabel = document.querySelector("[data-router-model]");
  const routerTypeLabel = document.querySelector("[data-router-type]");
  const modelTitle = document.querySelector("[data-model-title]");
  const modelSubtitle = document.querySelector("[data-model-subtitle]");
  const typingTitle = document.querySelector("[data-typing-title]");

  function sanitizeImageUrl(value) {
    if (typeof value !== "string") {
      return "";
    }

    const trimmed = value.trim();

    if (trimmed.startsWith("/") && !trimmed.startsWith("//")) {
      return trimmed;
    }

    if (trimmed.startsWith("data:image/")) {
      return trimmed;
    }

    try {
      const parsed = new URL(trimmed);
      const protocol = parsed.protocol.toLowerCase();

      if (protocol !== "http:" && protocol !== "https:") {
        return "";
      }

      return parsed.toString();
    } catch (_error) {
      return "";
    }
  }

  import("/hcai/planets.js?v=gpt56-planets-1")
    .then(function (module) {
      module.initPlanets(document);
    })
    .catch(function (error) {
      console.warn("[hcai] 3D planets unavailable:", error);
    });

  function updateFavicon(logoUrl) {
    if (!logoUrl) {
      return;
    }

    let link = document.querySelector('link[rel="icon"]');

    if (!link) {
      link = document.createElement("link");
      link.rel = "icon";
      document.head.appendChild(link);
    }

    link.type = logoUrl.endsWith(".svg") ? "image/svg+xml" : "image/x-icon";
    link.href = logoUrl;
  }

  function syncFaviconFromPublicSettings() {
    fetch("/api/v1/settings/public", { credentials: "same-origin" })
      .then(function (response) {
        return response.ok ? response.json() : null;
      })
      .then(function (payload) {
        const settings = payload && (payload.data || payload);
        updateFavicon(sanitizeImageUrl(settings && settings.site_logo));
      })
      .catch(function () {
        // Keep the existing favicon if public settings are unavailable.
      });
  }

  syncFaviconFromPublicSettings();

  function updateAnchorOffset() {
    if (!header) {
      return;
    }

    const offset = getAnchorOffset();
    document.documentElement.style.setProperty("--hc-anchor-offset", offset + "px");
  }

  updateAnchorOffset();
  window.addEventListener("load", updateAnchorOffset);

  function getAnchorOffset() {
    if (!header || window.getComputedStyle(header).position !== "sticky") {
      return 0;
    }

    return Math.ceil(header.getBoundingClientRect().height + 18);
  }

  function scrollToAnchor(target) {
    updateAnchorOffset();

    const targetTop = target.getBoundingClientRect().top + window.scrollY - getAnchorOffset();
    window.scrollTo({
      top: Math.max(0, targetTop),
      behavior: "smooth"
    });
  }

  function setMenu(open) {
    body.classList.toggle("hc-menu-open", open);

    if (menuToggle) {
      menuToggle.setAttribute("aria-expanded", String(open));
      menuToggle.setAttribute("aria-label", open ? "关闭菜单" : "打开菜单");
    }
  }

  if (menuToggle) {
    menuToggle.addEventListener("click", function () {
      setMenu(!body.classList.contains("hc-menu-open"));
    });
  }

  if (menuPanel) {
    menuPanel.addEventListener("click", function (event) {
      if (event.target.closest("a")) {
        setMenu(false);
      }
    });
  }

  document.addEventListener("click", function (event) {
    const link = event.target.closest('a[href^="#"]');

    if (!link) {
      return;
    }

    const hash = link.getAttribute("href");

    if (!hash || hash === "#") {
      return;
    }

    const target = document.querySelector(hash);

    if (!target) {
      return;
    }

    event.preventDefault();
    setMenu(false);
    history.pushState(null, "", window.location.pathname + window.location.search + hash);
    scrollToAnchor(target);
  });

  window.addEventListener("resize", function () {
    updateAnchorOffset();

    if (window.innerWidth > 860) {
      setMenu(false);
    }
  });

  if (closeNotice && notice) {
    closeNotice.addEventListener("click", function () {
      notice.classList.add("hc-notice--hidden");
      updateAnchorOffset();
    });
  }

  if (routerModels.length && routerModelLabel && routerTypeLabel) {
    let activeModelIndex = 0;

    function updateActiveModel(index) {
      routerModels.forEach(function (model, modelIndex) {
        model.classList.toggle("is-active", modelIndex === index);
      });

      const activeModel = routerModels[index];
      const modelName = activeModel.getAttribute("data-model-name") || activeModel.textContent;
      const modelType = activeModel.getAttribute("data-model-type") || "Model";

      routerModelLabel.textContent = modelName;
      routerTypeLabel.textContent = modelType;

      if (modelTitle) {
        modelTitle.textContent = activeModel.getAttribute("data-model-headline") || modelName;
      }

      if (modelSubtitle) {
        modelSubtitle.textContent = activeModel.getAttribute("data-model-copy") || "";
      }
    }

    updateActiveModel(activeModelIndex);

    window.setInterval(function () {
      activeModelIndex = (activeModelIndex + 1) % routerModels.length;
      updateActiveModel(activeModelIndex);
    }, 1800);
  }

  if (typingTitle && !window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
    const phrases = [
      "一个入口，调用所有 AI",
      "一个 Key，切换多模型",
      "国内直连，开箱即用",
      "Codex / Claude Code 友好"
    ];
    let phraseIndex = 0;
    let charIndex = 0;
    let deleting = false;
    const baseTypingSize = parseFloat(window.getComputedStyle(typingTitle).fontSize) || 72;
    const typingMeasure = document.createElement("span");

    typingMeasure.setAttribute("aria-hidden", "true");
    typingMeasure.style.position = "fixed";
    typingMeasure.style.left = "-9999px";
    typingMeasure.style.top = "0";
    typingMeasure.style.visibility = "hidden";
    typingMeasure.style.whiteSpace = "nowrap";
    typingMeasure.style.fontFamily = window.getComputedStyle(typingTitle).fontFamily;
    typingMeasure.style.fontWeight = window.getComputedStyle(typingTitle).fontWeight;
    typingMeasure.style.letterSpacing = window.getComputedStyle(typingTitle).letterSpacing;
    document.body.appendChild(typingMeasure);

    function fitTypingTitle() {
      typingTitle.style.setProperty("--hc-typing-size", baseTypingSize + "px");
      typingMeasure.style.fontSize = baseTypingSize + "px";
      typingMeasure.textContent = typingTitle.textContent;

      const titleContainer = typingTitle.parentElement || typingTitle;
      const availableWidth = Math.max(0, Math.min(titleContainer.clientWidth, typingTitle.clientWidth) - 28);
      const textWidth = typingMeasure.getBoundingClientRect().width;

      if (availableWidth > 0 && textWidth > availableWidth) {
        const nextSize = Math.max(24, baseTypingSize * (availableWidth / textWidth));
        typingTitle.style.setProperty("--hc-typing-size", nextSize + "px");
      }
    }

    function tickTypingTitle() {
      const phrase = phrases[phraseIndex];

      if (deleting) {
        charIndex -= 1;
      } else {
        charIndex += 1;
      }

      typingTitle.textContent = phrase.slice(0, charIndex);
      fitTypingTitle();

      if (!deleting && charIndex === phrase.length) {
        deleting = true;
        window.setTimeout(tickTypingTitle, 1500);
        return;
      }

      if (deleting && charIndex === 0) {
        deleting = false;
        phraseIndex = (phraseIndex + 1) % phrases.length;
      }

      window.setTimeout(tickTypingTitle, deleting ? 42 : 74);
    }

    typingTitle.textContent = "";
    fitTypingTitle();
    window.setTimeout(tickTypingTitle, 500);
    window.addEventListener("resize", fitTypingTitle);
  }

  const caseButtons = Array.from(document.querySelectorAll("[data-case-src]"));
  const caseFrame = document.querySelector("[data-case-frame]");
  const casePrompt = document.querySelector("[data-case-prompt-output]");

  if (caseButtons.length && caseFrame && casePrompt) {
    caseButtons.forEach(function (button) {
      button.addEventListener("click", function () {
        const source = button.getAttribute("data-case-src");
        const title = button.getAttribute("data-case-title") || button.textContent.trim();
        const prompt = button.getAttribute("data-case-prompt") || "";

        caseButtons.forEach(function (currentButton) {
          const active = currentButton === button;
          currentButton.classList.toggle("is-active", active);
          currentButton.setAttribute("aria-selected", String(active));
        });

        if (source && caseFrame.getAttribute("src") !== source) {
          caseFrame.setAttribute("src", source);
        }

        caseFrame.setAttribute("title", title);
        casePrompt.textContent = prompt;
        casePrompt.parentElement.hidden = !prompt;
      });
    });
  }

  faqItems.forEach(function (item) {
    const question = item.querySelector(".hc-faq__question");
    const answer = item.querySelector(".hc-faq__answer");

    if (!question || !answer) {
      return;
    }

    question.addEventListener("click", function () {
      const shouldOpen = !item.classList.contains("hc-faq__item--open");

      faqItems.forEach(function (currentItem) {
        const currentQuestion = currentItem.querySelector(".hc-faq__question");
        const currentAnswer = currentItem.querySelector(".hc-faq__answer");

        currentItem.classList.remove("hc-faq__item--open");

        if (currentQuestion) {
          currentQuestion.setAttribute("aria-expanded", "false");
        }

        if (currentAnswer) {
          currentAnswer.hidden = true;
        }
      });

      if (shouldOpen) {
        item.classList.add("hc-faq__item--open");
        question.setAttribute("aria-expanded", "true");
        answer.hidden = false;
      }
    });
  });
})();
