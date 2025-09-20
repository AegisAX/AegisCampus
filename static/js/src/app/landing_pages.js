/*
	landing_pages.js
	Handles the creation, editing, and deletion of landing pages
	Author: Jordan Wright <github.com/jordan-wright>
*/
var pages = []


// Save attempts to POST to /templates/
function save(idx) {
    var page = {}
    page.name = $("#name").val()
    editor = CKEDITOR.instances["html_editor"]
    page.html = editor.getData()
    page.capture_credentials = $("#capture_credentials_checkbox").prop("checked")
    page.capture_passwords = $("#capture_passwords_checkbox").prop("checked")
    page.redirect_url = $("#redirect_url_input").val()
    if (idx != -1) {
        page.id = pages[idx].id
        api.pageId.put(page)
            .success(function (data) {
                successFlash("Page edited successfully!")
                load()
                dismiss()
            })
    } else {
        // Submit the page
        api.pages.post(page)
            .success(function (data) {
                successFlash("Page added successfully!")
                load()
                dismiss()
            })
            .error(function (data) {
                modalError(data.responseJSON.message)
            })
    }
}

function dismiss() {
    $("#modal\\.flashes").empty()
    $("#name").val("")
    $("#html_editor").val("")
    $("#url").val("")
    $("#redirect_url_input").val("")
    $("#modal").find("input[type='checkbox']").prop("checked", false)
    $("#capture_passwords").hide()
    $("#redirect_url").hide()
    $("#modal").modal('hide')
}

var deletePage = function (idx) {
    Swal.fire({
        title: "Are you sure?",
        text: "This will delete the landing page. This can't be undone!",
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Delete " + escapeHtml(pages[idx].name),
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        preConfirm: function () {
            return new Promise(function (resolve, reject) {
                api.pageId.delete(pages[idx].id)
                    .success(function (msg) {
                        resolve()
                    })
                    .error(function (data) {
                        reject(data.responseJSON.message)
                    })
            })
        }
    }).then(function (result) {
        if (result.value){
            Swal.fire(
                'Landing Page Deleted!',
                'This landing page has been deleted!',
                'success'
            );
        }
        $('button:contains("OK")').on('click', function () {
            location.reload()
        })
    })
}

function importSite() {
    url = $("#url").val()
    if (!url) {
        modalError("No URL Specified!")
    } else {
        api.clone_site({
                url: url,
                include_resources: false
            })
            .success(function (data) {
                $("#html_editor").val(data.html)
                CKEDITOR.instances["html_editor"].setMode('wysiwyg')
                $("#importSiteModal").modal("hide")
            })
            .error(function (data) {
                modalError(data.responseJSON.message)
            })
    }
}

function edit(idx) {
    $("#modalSubmit").unbind('click').click(function () {
        save(idx)
    })
    $("#html_editor").ckeditor(null, {
      allowedContent: true,
      extraAllowedContent: 'script[*]{*}(*);video[*]{*}(*);source[*]{*}(*);style[*]{*}(*);iframe[*]{*}(*);div[*]{*}(*);span[*]{*}(*)'
    })
    setupAutocomplete(CKEDITOR.instances["html_editor"])

    var page = {}
    if (idx != -1) {
        $("#modalLabel").text("Edit Landing Page")
        page = pages[idx]
        $("#name").val(page.name)
        $("#html_editor").val(page.html)
        $("#capture_credentials_checkbox").prop("checked", page.capture_credentials)
        $("#capture_passwords_checkbox").prop("checked", page.capture_passwords)
        $("#redirect_url_input").val(page.redirect_url)
        if (page.capture_credentials) {
            $("#capture_passwords").show()
            $("#redirect_url").show()
        }
    } else {
        $("#modalLabel").text("New Landing Page")
    }
}

function copy(idx) {
    $("#modalSubmit").unbind('click').click(function () {
        save(-1)
    })
    $("#html_editor").ckeditor(null, {
      allowedContent: true,
      extraAllowedContent: 'script[*]{*}(*);video[*]{*}(*);source[*]{*}(*);style[*]{*}(*);iframe[*]{*}(*);div[*]{*}(*);span[*]{*}(*)'
    })
    var page = pages[idx]
    $("#name").val("Copy of " + page.name)
    $("#html_editor").val(page.html)
}

function load() {
    /*
        load() - Loads the current pages using the API
    */
    $("#pagesTable").hide()
    $("#emptyMessage").hide()
    $("#loading").show()
    api.pages.get()
        .success(function (ps) {
            pages = ps
            $("#loading").hide()
            if (pages.length > 0) {
                $("#pagesTable").show()
                pagesTable = $("#pagesTable").DataTable({
                    destroy: true,
                    columnDefs: [{
                        orderable: false,
                        targets: "no-sort"
                    }]
                });
                pagesTable.clear()
                pageRows = []
                $.each(pages, function (i, page) {
                    pageRows.push([
                        escapeHtml(page.name),
                        moment(page.modified_date).format('MMMM Do YYYY, h:mm:ss a'),
                        "<div class='pull-right'><span data-toggle='modal' data-backdrop='static' data-target='#modal'><button class='btn btn-primary' data-toggle='tooltip' data-placement='left' title='Edit Page' onclick='edit(" + i + ")'>\
                    <i class='fa fa-pencil'></i>\
                    </button></span>\
		    <span data-toggle='modal' data-target='#modal'><button class='btn btn-primary' data-toggle='tooltip' data-placement='left' title='Copy Page' onclick='copy(" + i + ")'>\
                    <i class='fa fa-copy'></i>\
                    </button></span>\
                    <button class='btn btn-danger' data-toggle='tooltip' data-placement='left' title='Delete Page' onclick='deletePage(" + i + ")'>\
                    <i class='fa fa-trash-o'></i>\
                    </button></div>"
                    ])
                })
                pagesTable.rows.add(pageRows).draw()
                $('[data-toggle="tooltip"]').tooltip()
            } else {
                $("#emptyMessage").show()
            }
        })
        .error(function () {
            $("#loading").hide()
            errorFlash("Error fetching pages")
        })
}

$(document).ready(function () {
    // CKEditor가 script, video, source, style, iframe 등을 지우지 않도록 허용
    if (window.CKEDITOR) {
      CKEDITOR.config.allowedContent = true;
      CKEDITOR.config.extraAllowedContent = 'script[*]{*}(*);video[*]{*}(*);source[*]{*}(*);style[*]{*}(*);iframe[*]{*}(*);div[*]{*}(*);span[*]{*}(*)';
    }
    // Setup multiple modals
    // Code based on http://miles-by-motorcycle.com/static/bootstrap-modal/index.html
    $('.modal').on('hidden.bs.modal', function (event) {
        $(this).removeClass('fv-modal-stack');
        $('body').data('fv_open_modals', $('body').data('fv_open_modals') - 1);
    });
    $('.modal').on('shown.bs.modal', function (event) {
        // Keep track of the number of open modals
        if (typeof ($('body').data('fv_open_modals')) == 'undefined') {
            $('body').data('fv_open_modals', 0);
        }
        // if the z-index of this modal has been set, ignore.
        if ($(this).hasClass('fv-modal-stack')) {
            return;
        }
        $(this).addClass('fv-modal-stack');
        // Increment the number of open modals
        $('body').data('fv_open_modals', $('body').data('fv_open_modals') + 1);
        // Setup the appropriate z-index
        $(this).css('z-index', 1040 + (10 * $('body').data('fv_open_modals')));
        $('.modal-backdrop').not('.fv-modal-stack').css('z-index', 1039 + (10 * $('body').data('fv_open_modals')));
        $('.modal-backdrop').not('fv-modal-stack').addClass('fv-modal-stack');
    });
    $.fn.modal.Constructor.prototype.enforceFocus = function () {
        $(document)
            .off('focusin.bs.modal') // guard against infinite focus loop
            .on('focusin.bs.modal', $.proxy(function (e) {
                if (
                    this.$element[0] !== e.target && !this.$element.has(e.target).length
                    // CKEditor compatibility fix start.
                    &&
                    !$(e.target).closest('.cke_dialog, .cke').length
                    // CKEditor compatibility fix end.
                ) {
                    this.$element.trigger('focus');
                }
            }, this));
    };
    // Scrollbar fix - https://stackoverflow.com/questions/19305821/multiple-modals-overlay
    $(document).on('hidden.bs.modal', '.modal', function () {
        $('.modal:visible').length && $(document.body).addClass('modal-open');
    });
    $('#modal').on('hidden.bs.modal', function (event) {
        dismiss()
    });
    $("#capture_credentials_checkbox").change(function () {
        $("#capture_passwords").toggle()
        $("#redirect_url").toggle()
    })
    CKEDITOR.on('dialogDefinition', function (ev) {
        // Take the dialog name and its definition from the event data.
        var dialogName = ev.data.name;
        var dialogDefinition = ev.data.definition;

        // Check if the definition is from the dialog window you are interested in (the "Link" dialog window).
        if (dialogName == 'link') {
            dialogDefinition.minWidth = 500
            dialogDefinition.minHeight = 100

            // Remove the linkType field
            var infoTab = dialogDefinition.getContents('info');
            infoTab.get('linkType').hidden = true;
        }
    });

    load()
})

/* === Video Picker Integration for Landing Pages === */

// 호출: "Insert Training Video" 버튼 클릭 시
function showVideoPicker() {
    // Open modal and load videos
    $("#videoPickerModal").modal("show");
    loadVideoPicker();
}

function loadVideoPicker() {
    $("#videoPickerLoading").show();
    $("#videoPickerTable").hide();
    $("#videoPickerEmpty").hide();
    $("#videoPickerTable tbody").empty();

    api.videos.get()
        .success(function (videos) {
            $("#videoPickerLoading").hide();
            if (!Array.isArray(videos) || videos.length === 0) {
                $("#videoPickerEmpty").show();
                return;
            }
            $("#videoPickerTable").show();
            videos.forEach(function (v) {
                var row = $("<tr>");
                row.append($("<td>").text(v.name));
                var uploaded = v.modified_date ? moment(v.modified_date).format('YYYY-MM-DD HH:mm') : "";
                row.append($("<td>").text(uploaded));
                var insertBtn = $("<button class='btn btn-primary btn-sm'>Insert</button>");
                insertBtn.on("click", function () {
                    insertTrainingTemplate(v.Id || v.id, v.name); // 서버 모델에 따라 Id 또는 id 사용
                    $("#videoPickerModal").modal("hide");
                });
                row.append($("<td>").append(insertBtn));
                $("#videoPickerTable tbody").append(row);
            });
        })
        .error(function () {
            $("#videoPickerLoading").hide();
            modalError("Error fetching videos");
        });
}

// CKEditor에 "완성된 학습용 랜딩 페이지" 템플릿을 통으로 세팅 (이어보기 안정화 버전)
function insertTrainingTemplate(videoId, videoName) {
  var editor = CKEDITOR.instances["html_editor"];
  var hasContent = false;
  try {
    if (editor) hasContent = (editor.getData() || "").trim().length > 0;
    else {
      var ta = document.getElementById("html_editor");
      hasContent = ta && (ta.value || "").trim().length > 0;
    }
  } catch (_) {}

  if (hasContent) {
    if (!confirm("현재 내용을 모두 교체하고 학습용 템플릿을 삽입할까요?")) return;
  }

  var safeTitle = (videoName || "사이버보안 인식 제고 교육").replace(/</g,"&lt;").replace(/>/g,"&gt;");
  var html = [
'<!DOCTYPE html>',
'<html lang="ko">',
'<head>',
'  <meta charset="UTF-8">',
'  <title>' + safeTitle + '</title>',
'  <meta name="viewport" content="width=device-width, initial-scale=1">',
'  <link href="https://cdn.jsdelivr.net/npm/bootstrap@3.4.1/dist/css/bootstrap.min.css" rel="stylesheet">',
'  <style>',
'    body { background: #f7f9fc; }',
'    header { background:#0f2743; color:#fff; padding:15px; text-align:center; }',
'    .container-box { max-width:960px; margin:30px auto; background:#fff; padding:25px; border-radius:12px; box-shadow:0 6px 18px rgba(0,0,0,0.08);}',
'    #training-video { width:100%; border-radius:8px; box-shadow:0 4px 12px rgba(0,0,0,0.2);}',
'    #progress-bar { height:22px; line-height:22px; }',
'    .btn-complete { margin-top:20px; }',
'    video::-internal-media-controls-download-button { display: none; }',
'    video::-webkit-media-controls-enclosure { overflow: hidden; }',
'    video::-webkit-media-controls-picture-in-picture-button { display: none; }',
'  </style>',
'</head>',
'<body>',
'  <header>',
'    <h2><i class="glyphicon glyphicon-education"></i> 사이버보안 인식 제고 교육</h2>',
'  </header>',
'  <div class="container-box">',
'    <video id="training-video" controls preload="metadata" playsinline disablePictureInPicture controlsList="nodownload noplaybackrate" oncontextmenu="return false">',
'      <source src="/media/' + (typeof videoId === "number" ? videoId : String(videoId)) + '" type="video/mp4">',
'      지원되지 않는 브라우저입니다.',
'    </video>',
'    <div id="resumeBadge" class="text-muted" style="display:none;margin-top:8px;font-size:12px;opacity:.8;"></div>',
'    <!-- div class="progress" style="margin-top:12px;">',
'      <div id="progress-bar" class="progress-bar progress-bar-success" role="progressbar" style="width:0%">0%</div>',
'    </div -->',
'    <p style="font-size:18px;margin-top:10px;"><strong>' + safeTitle + '</strong></p>',
'    <p style="font-size:18px;">* 교육 수강 완료 후 반드시 아래 <strong style="color:red;">수강 완료 확인</strong> 버튼을 클릭하세요.</p>',
'    <button id="complete-btn" class="btn btn-success btn-lg btn-block btn-complete" disabled>수강 완료 확인</button>',
'  </div>',
'',
'  <script>',
'  (function(){',
'    var v = document.getElementById("training-video");',
'    var bar = document.getElementById("progress-bar");',
'    var badge = document.getElementById("resumeBadge");',
'    var completeBtn = document.getElementById("complete-btn");',
'    var params = new URLSearchParams(location.search);',
'    var rid = params.get("rid") || params.get("RID") || "";',
'    var videoId = ' + (typeof videoId === "number" ? videoId : JSON.stringify(String(videoId))) + ';',
'',
'    // ---- 설정 ----',
'    var SAVE_INTERVAL_MS = 5000;',
'    var MIN_DELTA_SECONDS = 2;',
'    var READ_URLS = [',
'      function(rid, vid){ return "/track/video/progress?rid=" + encodeURIComponent(rid) + "&video_id=" + encodeURIComponent(vid); },',
'      function(rid, vid){ return "/track/video?op=progress&rid=" + encodeURIComponent(rid) + "&video_id=" + encodeURIComponent(vid); }',
'    ];',
'    var WRITE_URL = "/track/video";',
'    var localKey = "vh_progress:" + videoId + ":" + rid;',
'',
'    // ---- 상태 ----',
'    var lastSentSec = -1;',
'    var sendLock = false;',
'    var saveTimer = null;',
'    var allowMax = 0;',
'    var CLAMP_EPS = 0.25;',
'    var clampBusy = false;',
'    var resumeAt = null;             // 서버/로컬에서 받아올 목표 재개 지점',
'    var resumeApplied = false;       // 실제 적용 완료 여부',
'',
'    function fmt(sec){ sec=Math.max(0,Math.floor(sec||0)); var m=(sec/60|0).toString().padStart(2,"0"); var s=(sec%60).toString().padStart(2,"0"); return m+":"+s; }',
'    function readyForSeek(){ return v && v.readyState >= 1; } // HAVE_METADATA 이상',
'',
'    // --- (핵심) 재개 적용: 메타데이터 준비/재개값 확보 중 어느 쪽이 먼저여도 동작 ---',
'    function applyResumeIfReady(){',
'      if (resumeApplied) return;',
'      if (resumeAt == null) return;',          // 아직 재개값 모름',
'      if (!readyForSeek()) return;',           // 아직 메타데이터 미준비',
'      if (!(resumeAt > 0) || resumeAt >= (v.duration || Infinity)) {',
'        // 재개할 필요 없음',
'        resumeApplied = true;',
'        allowMax = Math.max(allowMax, v.currentTime||0);',
'        return;',
'      }',
'      // 클램프가 되돌리지 않도록 보호 + allowMax를 선반영',
'      clampBusy = true;',
'      allowMax = Math.max(allowMax, resumeAt);',
'      try {',
'        v.currentTime = resumeAt;',
'        badge.textContent = "마지막 시청 위치(" + fmt(resumeAt) + ")로 이동했습니다.";',
'        badge.style.display = "block";',
'        setTimeout(function(){ badge.style.display = "none"; }, 4000);',
'      } catch(e) {}',
'      clampBusy = false;',
'      resumeApplied = true;',
'    }',
'',
'    // --- 진행률 조회 (동시에 시작, 완료되면 재시도 없이 applyResumeIfReady 호출) ---',
'    (function fetchServerProgress(){',
'      if (!rid || !videoId) { resumeAt = loadLocal(); applyResumeIfReady(); return; }',
'      var tried = 0;',
'      function next(){',
'        if (tried >= READ_URLS.length) { resumeAt = Math.max(0, loadLocal()); applyResumeIfReady(); return; }',
'        var url = READ_URLS[tried++](rid, videoId);',
'        fetch(url, { method:"GET", credentials:"same-origin" })',
'          .then(function(res){ if(!res.ok) throw 0; return res.json(); })',
'          .then(function(data){',
'            var sv = (data && typeof data.seconds_watched === "number") ? data.seconds_watched : 0;',
'            resumeAt = Math.max(0, sv, loadLocal());',
'            applyResumeIfReady();',
'          })',
'          .catch(function(){ next(); });',
'      }',
'      next();',
'    })();',
'',
'    // --- 메타데이터가 더 빨리 준비되는 경우 대비: 리스너를 "즉시" 먼저 단다 ---',
'    v.addEventListener("loadedmetadata", applyResumeIfReady);',
'    // 이미 메타데이터 준비 상태로 로드된 경우(빠른 캐시) 즉시 시도',
'    if (readyForSeek()) setTimeout(applyResumeIfReady, 0);',
'    // Safari 등에서 재생 버튼을 눌렀을 때도 보정',
'    v.addEventListener("play", applyResumeIfReady, { once: true });',
'',
'    // ---- 클램프 & UI ----',
'    function clampForward(){',
'      if (clampBusy) return;',
'      var cur = v.currentTime || 0;',
'      if (cur > allowMax + CLAMP_EPS) {',
'        clampBusy = true;',
'        v.currentTime = allowMax;',
'        if (!v.paused) v.play().catch(function(){});',
'        clampBusy = false;',
'      }',
'    }',
'    function updateUI(){',
'      if (!v.duration) return;',
'      var p = (v.currentTime / v.duration) * 100;',
'      bar.style.width = p.toFixed(1) + "%";',
'      bar.textContent = p.toFixed(0) + "%";',
'    }',
'',
'    // ---- 저장 ----',
'    function loadLocal(){',
'      try { var raw = localStorage.getItem(localKey); if(!raw) return 0; var p=JSON.parse(raw); return (p && Number.isFinite(p.seconds_watched)) ? p.seconds_watched : 0; } catch(e){ return 0; }',
'    }',
'    async function sendProgress(opts){',
'      opts = opts || {};',
'      if (!rid || !videoId) return;',
'      if (sendLock) return;',
'      var cur = Math.floor(v.currentTime || 0);',
'      var dur = Math.floor(v.duration || 0);',
'      var must = !!opts.force;',
'      var completed = !!opts.completed;',
'      if (!must && Math.abs(cur - lastSentSec) < MIN_DELTA_SECONDS) return;',
'      var payload = {',
'        rid: rid,',
'        video_id: (typeof videoId === "string" ? videoId : Number(videoId)),',
'        seconds_watched: cur,',
'        duration: dur || undefined,',
'        completed: completed',
'      };',
'      try {',
'        sendLock = true;',
'        var res = await fetch(WRITE_URL, {',
'          method:"POST", headers:{"Content-Type":"application/json"}, credentials:"same-origin", body: JSON.stringify(payload)',
'        });',
'        if (res.ok) {',
'          lastSentSec = cur;',
'          localStorage.setItem(localKey, JSON.stringify({ seconds_watched: cur, duration: dur }));',
'        }',
'      } catch(e){',
'        localStorage.setItem(localKey, JSON.stringify({ seconds_watched: cur, duration: dur }));',
'      } finally { sendLock = false; }',
'    }',
'    function beaconFlush(done){',
'      if (!rid || !videoId) return;',
'      var cur = Math.floor(v.currentTime || 0);',
'      var dur = Math.floor(v.duration || 0);',
'      var payload = JSON.stringify({',
'        rid: rid,',
'        video_id: (typeof videoId === "string" ? videoId : Number(videoId)),',
'        seconds_watched: cur,',
'        duration: dur || undefined,',
'        completed: !!done',
'      });',
'      try {',
'        var blob = new Blob([payload], { type:"application/json" });',
'        if (!navigator.sendBeacon || !navigator.sendBeacon(WRITE_URL, blob)) {',
'          fetch(WRITE_URL, { method:"POST", headers:{"Content-Type":"application/json"}, credentials:"same-origin", body: payload, keepalive:true }).catch(function(){});',
'        }',
'      } catch(e){}',
'      localStorage.setItem(localKey, JSON.stringify({ seconds_watched: cur, duration: dur }));',
'    }',
'    function startAutoSave(){',
'      if (saveTimer) clearInterval(saveTimer);',
'      saveTimer = setInterval(function(){',
'        if (!v.paused && !v.seeking && v.readyState >= 1) sendProgress();',
'      }, SAVE_INTERVAL_MS);',
'    }',
'',
'    // ---- 이벤트 ----',
'    v.addEventListener("timeupdate", function(){',
'      if (!v.seeking && !clampBusy) { if (v.currentTime > allowMax) allowMax = v.currentTime; }',
'      clampForward();',
'      updateUI();',
'    });',
'    v.addEventListener("seeking", clampForward);',
'    v.addEventListener("seeked",  clampForward);',
'',
'    v.addEventListener("play",  function(){ sendProgress({force:true}); });',
'    v.addEventListener("pause", function(){ sendProgress({force:true}); });',
'    v.addEventListener("ended", function(){',
'      sendProgress({force:true, completed:true});',
'      beaconFlush(true);',
'      completeBtn.disabled = false;',
'    });',
'    completeBtn.addEventListener("click", function(){ alert("수강 완료가 확인되었습니다. 감사합니다!"); });',
'    window.addEventListener("beforeunload", function(){ beaconFlush(false); });',
'    document.addEventListener("visibilitychange", function(){ if (document.visibilityState === "hidden") beaconFlush(false); });',
'    window.addEventListener("online", function(){ sendProgress({force:true}); });',
'',
'    // ---- 실행 ----',
'    startAutoSave();',
'  })();',
'  </script>',
'</body>',
'</html>'
  ].join("\n");

  try {
    if (editor) {
      editor.setData(html);
      editor.setMode('wysiwyg');
      editor.focus();
    } else {
      var ta2 = document.getElementById("html_editor");
      if (ta2) ta2.value = html;
    }
    successFlash("학습용 랜딩 페이지 템플릿(이어보기 안정화)을 삽입했습니다.");
  } catch (e) {
    console.error(e);
    modalError("템플릿 삽입에 실패했습니다.");
  }
}

