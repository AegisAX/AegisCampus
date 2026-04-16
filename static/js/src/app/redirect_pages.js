/*
    redirect_pages.js
    Handles the creation, editing, and deletion of redirect pages
    Based on landing_pages.js
*/
var pages = []
var selectedVideoId = null;
/* === 피싱 페이지 내장용 미니 Swal (외부 의존성 없음) === */
var MINI_SWAL = '(function(){' +
    'var _sf=function(o){return new Promise(function(res){' +
    'var ov=document.createElement("div");' +
    'ov.style.cssText="position:fixed;inset:0;background:rgba(0,0,0,.45);display:flex;align-items:center;justify-content:center;z-index:99999";' +
    'var ic={"success":"\u2705","error":"\u274c","warning":"\u26a0\ufe0f","info":"\u2139\ufe0f"}[o.icon||""]||"";' +
    'ov.innerHTML="<div style=\\"background:#fff;border-radius:12px;padding:32px 24px;max-width:360px;width:90%;text-align:center;box-shadow:0 8px 32px rgba(0,0,0,.18)\\">"' +
    '+"<div style=\\"font-size:48px;margin-bottom:12px\\">"+ic+"</div>"' +
    '+(o.title?"<h2 style=\\"margin:0 0 10px;font-size:22px;color:#2c3e50\\">"+o.title+"</h2>":"")' +
    '+(o.text?"<p style=\\"margin:0 0 20px;color:#555;font-size:15px\\">"+o.text+"</p>":"")' +
    '+"<button style=\\"background:#3085d6;color:#fff;border:0;border-radius:8px;padding:10px 28px;font-size:16px;font-weight:700;cursor:pointer\\">"+(o.confirmButtonText||"OK")+"</button>"' +
    '+"</div>";' +
    'ov.querySelector("button").onclick=function(){document.body.removeChild(ov);res({value:true});};' +
    'document.body.appendChild(ov);' +
    '});};' +
    'window.Swal={fire:_sf};' +
    '})();';

// Save attempts to POST/PUT to /redirect_pages/
function save(idx) {
    var page = {}
    page.name = $("#name").val()
    var editor = CKEDITOR.instances["html_editor"]
    page.html = editor ? editor.getData() : ""   // null 방어
    page.redirect_url = $("#redirect_url_input").val()
    // 수정 — HTML에서 직접 추출 (selectedVideoId보다 우선)
    page.video_id = extractVideoIdFromHtml(
        editor ? editor.getData() : ""
    );
    if (idx != -1) {
        page.id = pages[idx].id
        api.redirectPageId.put(page)
            .success(function (data) {
                successFlash("Page edited successfully!")
                load()
                dismiss()
            })
            .error(function (data) {
                modalError(data.responseJSON.message)
            })
    } else {
        api.redirectPages.post(page)
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
    if (CKEDITOR.instances["html_editor"]) {
        CKEDITOR.instances["html_editor"].setData("")
    }
    $("#url").val("")
    $("#redirect_url_input").val("")
    $("#modal").modal('hide')
}

var deletePage = function (idx) {
    Swal.fire({
        title: "Are you sure?",
        text: "This will delete the redirect page. This can't be undone!",
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Delete " + escapeHtml(pages[idx].name),
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        preConfirm: function () {
            return new Promise(function (resolve, reject) {
                api.redirectPageId.delete(pages[idx].id)
                    .success(function (msg) {
                        resolve()
                    })
                    .error(function (data) {
                        reject(data.responseJSON.message)
                    })
            })
        }
    }).then(function (result) {
        if (result.value) {
            Swal.fire(
                'Redirect Page Deleted!',
                'This redirect page has been deleted!',
                'success'
            );
        }
        $('button:contains("OK")').on('click', function () {
            location.reload()
        })
    })
}

function importSite() {
    var url = $("#url").val()
    if (!url) {
        modalError("No URL Specified!")
    } else {
        api.clone_site({
                url: url,
                include_resources: false
            })
            .success(function (data) {
                CKEDITOR.instances["html_editor"].setData(data.html)
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

    var page = {}
    if (idx != -1) {
        $("#modalLabel").text("Edit Redirect Page")
        page = pages[idx]
        $("#name").val(page.name)
        $("#redirect_url_input").val(page.redirect_url)
        selectedVideoId = (page.video_id !== undefined && page.video_id !== null)
            ? page.video_id : null;   // ← 추가: 기존 video_id 복원
    } else {
        $("#modalLabel").text("New Redirect Page")
        selectedVideoId = null;       // ← 추가: 신규 생성 시 초기화
    }

    var html = (idx != -1 && page.html) ? page.html : ''

    if (CKEDITOR.instances["html_editor"]) {
        CKEDITOR.instances["html_editor"].setData(html)
    } else {
        $("#html_editor").ckeditor(null, {
            allowedContent: true,
            extraAllowedContent: 'script[*]{*}(*);video[*]{*}(*);source[*]{*}(*);style[*]{*}(*);iframe[*]{*}(*);div[*]{*}(*);span[*]{*}(*)'
        })
        CKEDITOR.instances["html_editor"].on('instanceReady', function () {
            this.setData(html)
        })
    }

    setupAutocomplete(CKEDITOR.instances["html_editor"])
}

function copy(idx) {
    $("#modalSubmit").unbind('click').click(function () {
        save(-1)
    })

    var page = pages[idx]
    $("#name").val("Copy of " + page.name)
    var html = page.html || ''

    if (CKEDITOR.instances["html_editor"]) {
        CKEDITOR.instances["html_editor"].setData(html)
    } else {
        $("#html_editor").ckeditor(null, {
            allowedContent: true,
            extraAllowedContent: 'script[*]{*}(*);video[*]{*}(*);source[*]{*}(*);style[*]{*}(*);iframe[*]{*}(*);div[*]{*}(*);span[*]{*}(*)'
        })
        CKEDITOR.instances["html_editor"].on('instanceReady', function () {
            this.setData(html)
        })
    }
}

function load() {
    $("#redirectPagesTable").hide()
    $("#emptyMessage").hide()
    $("#loading").show()
    api.redirectPages.get()
        .success(function (ps) {
            pages = ps
            $("#loading").hide()
            if (pages.length > 0) {
                $("#redirectPagesTable").show()
                var redirectPagesTable = $("#redirectPagesTable").DataTable({
                    destroy: true,
                    columnDefs: [{
                        orderable: false,
                        targets: "no-sort"
                    }]
                });
                redirectPagesTable.clear()
                var pageRows = []
                $.each(pages, function (i, page) {
                    pageRows.push([
                        escapeHtml(page.name),
                        moment(page.modified_date).format('MMMM Do YYYY, h:mm:ss a'),
                        "<div class='pull-right'>\
                            <span data-toggle='modal' data-backdrop='static' data-target='#modal'>\
                                <button class='btn btn-primary' data-toggle='tooltip' data-placement='left' title='Edit Page' onclick='edit(" + i + ")'>\
                                    <i class='fa fa-pencil'></i>\
                                </button>\
                            </span>\
                            <span data-toggle='modal' data-target='#modal'>\
                                <button class='btn btn-primary' data-toggle='tooltip' data-placement='left' title='Copy Page' onclick='copy(" + i + ")'>\
                                    <i class='fa fa-copy'></i>\
                                </button>\
                            </span>\
                            <button class='btn btn-danger' data-toggle='tooltip' data-placement='left' title='Delete Page' onclick='deletePage(" + i + ")'>\
                                <i class='fa fa-trash-o'></i>\
                            </button>\
                        </div>"
                    ])
                })
                redirectPagesTable.rows.add(pageRows).draw()
                $('[data-toggle="tooltip"]').tooltip()
            } else {
                $("#emptyMessage").show()
            }
        })
        .error(function () {
            $("#loading").hide()
            errorFlash("Error fetching redirect pages")
        })
}

$(document).ready(function () {
    if (window.CKEDITOR) {
        CKEDITOR.config.allowedContent = true;
        CKEDITOR.config.extraAllowedContent = 'script[*]{*}(*);video[*]{*}(*);source[*]{*}(*);style[*]{*}(*);iframe[*]{*}(*);div[*]{*}(*);span[*]{*}(*)';
    }
    $('.modal').on('hidden.bs.modal', function (event) {
        $(this).removeClass('fv-modal-stack');
        $('body').data('fv_open_modals', $('body').data('fv_open_modals') - 1);
    });
    $('.modal').on('shown.bs.modal', function (event) {
        if (typeof ($('body').data('fv_open_modals')) == 'undefined') {
            $('body').data('fv_open_modals', 0);
        }
        if ($(this).hasClass('fv-modal-stack')) {
            return;
        }
        $(this).addClass('fv-modal-stack');
        $('body').data('fv_open_modals', $('body').data('fv_open_modals') + 1);
        $(this).css('z-index', 1040 + (10 * $('body').data('fv_open_modals')));
        $('.modal-backdrop').not('.fv-modal-stack').css('z-index', 1039 + (10 * $('body').data('fv_open_modals')));
        $('.modal-backdrop').not('fv-modal-stack').addClass('fv-modal-stack');
    });
    $.fn.modal.Constructor.prototype.enforceFocus = function () {
        $(document)
            .off('focusin.bs.modal')
            .on('focusin.bs.modal', $.proxy(function (e) {
                if (
                    this.$element[0] !== e.target && !this.$element.has(e.target).length &&
                    !$(e.target).closest('.cke_dialog, .cke').length
                ) {
                    this.$element.trigger('focus');
                }
            }, this));
    };
    $(document).on('hidden.bs.modal', '.modal', function () {
        $('.modal:visible').length && $(document.body).addClass('modal-open');
    });
    $('#modal').on('hidden.bs.modal', function (event) {
        dismiss()
    });
    CKEDITOR.on('dialogDefinition', function (ev) {
        var dialogName = ev.data.name;
        var dialogDefinition = ev.data.definition;
        if (dialogName == 'link') {
            dialogDefinition.minWidth = 500
            dialogDefinition.minHeight = 100
            var infoTab = dialogDefinition.getContents('info');
            infoTab.get('linkType').hidden = true;
        }
    });

    load()
})

/* === Video Picker Integration === */

function showVideoPicker() {
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
                    insertTrainingTemplate(v.Id || v.id, v.name);
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

/* === video_id 추출 유틸 === */
function extractVideoIdFromHtml(html) {
    if (!html) return null;
    var match = html.match(/\/media\/(\d+)/);
    if (match) return parseInt(match[1], 10);
    return null;
}

function insertTrainingTemplate(videoId, videoName) {
    selectedVideoId = videoId;
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

    var safeTitle = (videoName || "사이버보안 인식 제고 교육").replace(/</g, "&lt;").replace(/>/g, "&gt;");
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
        '    .container-box { max-width:640px; margin:30px auto; background:#fff; padding:25px; border-radius:12px; box-shadow:0 6px 18px rgba(0,0,0,0.08);}',
        '    #training-video { width:100%; border-radius:8px; box-shadow:0 4px 12px rgba(0,0,0,0.2);}',
        '    .btn-complete { margin-top:20px; }',
        '    video::-internal-media-controls-download-button { display: none; }',
        '    video::-webkit-media-controls-enclosure { overflow: hidden; }',
        '    video::-webkit-media-controls-picture-in-picture-button { display: none; }',
        '  </style>',
        '  <script>' + MINI_SWAL + '<\/script>',
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
        '    <p style="font-size:18px;margin-top:10px;"><strong>' + safeTitle + '</strong></p>',
        '    <p style="font-size:18px;">* 교육 수강 완료 후 반드시 아래 <strong style="color:red;">수강 완료 확인</strong> 버튼을 클릭하세요.</p>',
        '    <button id="complete-btn" class="btn btn-success btn-lg btn-block btn-complete" disabled>수강 완료 확인</button>',
        '    <div id="complete-status" class="text-success" style="margin-top:8px; display:none;"></div>',
        '  </div>',
        '',
        '  <script>',
        '  (function(){',
        '    var v = document.getElementById("training-video");',
        '    var badge = document.getElementById("resumeBadge");',
        '    var completeBtn = document.getElementById("complete-btn");',
        '    var params = new URLSearchParams(location.search);',
        '    var rid = params.get("rid") || params.get("RID") || "";',
        '    var videoId = ' + (typeof videoId === "number" ? videoId : JSON.stringify(String(videoId))) + ';',
        '    var TRAINING_COMPLETE_URL = "/api/training/complete";',
        '    var SAVE_INTERVAL_MS = 5000;',
        '    var MIN_DELTA_SECONDS = 2;',
        '    var WRITE_URL = "/track/video";',
        '    var localKey = "vh_progress:" + videoId + ":" + rid;',
        '    var lastSentSec = -1;',
        '    var sendLock = false;',
        '    var saveTimer = null;',
        '    var allowMax = 0;',
        '    var CLAMP_EPS = 0.25;',
        '    var clampBusy = false;',
        '    var resumeAt = null;',
        '    var resumeApplied = false;',
        '',
        '    function fmt(sec){ sec=Math.max(0,Math.floor(sec||0)); var m=(sec/60|0).toString().padStart(2,"0"); var s=(sec%60).toString().padStart(2,"0"); return m+":"+s; }',
        '    function readyForSeek(){ return v && v.readyState >= 1; }',
        '',
        '    function applyResumeIfReady(){',
        '      if (resumeApplied) return;',
        '      if (resumeAt == null) return;',
        '      if (!readyForSeek()) return;',
        '      if (!(resumeAt > 0) || resumeAt >= (v.duration || Infinity)) { resumeApplied = true; allowMax = Math.max(allowMax, v.currentTime||0); return; }',
        '      clampBusy = true;',
        '      allowMax = Math.max(allowMax, resumeAt);',
        '      try { v.currentTime = resumeAt; badge.textContent = "마지막 시청 위치("+fmt(resumeAt)+")로 이동했습니다."; badge.style.display="block"; setTimeout(function(){ badge.style.display="none"; }, 4000); } catch(e) {}',
        '      clampBusy = false;',
        '      resumeApplied = true;',
        '    }',
        '',
        '    function loadLocal(){ try { var raw=localStorage.getItem(localKey); if(!raw) return 0; var p=JSON.parse(raw); return (p&&Number.isFinite(p.seconds_watched))?p.seconds_watched:0; } catch(e){ return 0; } }',
        '',
        '    (function fetchServerProgress(){',
        '      if (!rid || !videoId) { resumeAt = loadLocal(); applyResumeIfReady(); return; }',
        '      fetch("/track/video/progress?rid="+encodeURIComponent(rid)+"&video_id="+encodeURIComponent(videoId), {method:"GET",credentials:"same-origin"})',
        '        .then(function(res){ if(!res.ok) throw 0; return res.json(); })',
        '        .then(function(data){ resumeAt=Math.max(0,(data&&typeof data.seconds_watched==="number"?data.seconds_watched:0),loadLocal()); applyResumeIfReady(); })',
        '        .catch(function(){ resumeAt=Math.max(0,loadLocal()); applyResumeIfReady(); });',
        '    })();',
        '',
        '    v.addEventListener("loadedmetadata", applyResumeIfReady);',
        '    if (readyForSeek()) setTimeout(applyResumeIfReady, 0);',
        '    v.addEventListener("play", applyResumeIfReady, { once: true });',
        '',
        '    function clampForward(){ if(clampBusy)return; var cur=v.currentTime||0; if(cur>allowMax+CLAMP_EPS){ clampBusy=true; v.currentTime=allowMax; if(!v.paused)v.play().catch(function(){}); clampBusy=false; } }',
        '',
        '    async function sendProgress(opts){',
        '      opts=opts||{}; if(!rid||!videoId)return; if(sendLock)return;',
        '      var cur=Math.floor(v.currentTime||0); var dur=Math.floor(v.duration||0);',
        '      if(!opts.force && Math.abs(cur-lastSentSec)<MIN_DELTA_SECONDS) return;',
        '      var payload={rid:rid,video_id:(typeof videoId==="string"?videoId:Number(videoId)),seconds_watched:cur,duration:dur||undefined,completed:!!opts.completed};',
        '      try{ sendLock=true; var res=await fetch(WRITE_URL,{method:"POST",headers:{"Content-Type":"application/json"},credentials:"same-origin",body:JSON.stringify(payload)}); if(res.ok){lastSentSec=cur;localStorage.setItem(localKey,JSON.stringify({seconds_watched:cur,duration:dur}));} }catch(e){localStorage.setItem(localKey,JSON.stringify({seconds_watched:cur,duration:dur}));} finally{sendLock=false;}',
        '    }',
        '    function beaconFlush(done){ if(!rid||!videoId)return; var cur=Math.floor(v.currentTime||0); var dur=Math.floor(v.duration||0); var payload=JSON.stringify({rid:rid,video_id:(typeof videoId==="string"?videoId:Number(videoId)),seconds_watched:cur,duration:dur||undefined,completed:!!done}); try{var blob=new Blob([payload],{type:"application/json"}); if(!navigator.sendBeacon||!navigator.sendBeacon(WRITE_URL,blob)){fetch(WRITE_URL,{method:"POST",headers:{"Content-Type":"application/json"},credentials:"same-origin",body:payload,keepalive:true}).catch(function(){});}}catch(e){} localStorage.setItem(localKey,JSON.stringify({seconds_watched:cur,duration:dur})); }',
        '',
        '    v.addEventListener("timeupdate", function(){ if(!v.seeking&&!clampBusy){if(v.currentTime>allowMax)allowMax=v.currentTime;} clampForward(); });',
        '    v.addEventListener("seeking", clampForward);',
        '    v.addEventListener("seeked", clampForward);',
        '    v.addEventListener("play", function(){ sendProgress({force:true}); });',
        '    v.addEventListener("pause", function(){ sendProgress({force:true}); });',
        '    v.addEventListener("ended", function(){ sendProgress({force:true,completed:true}); beaconFlush(true); completeBtn.disabled=false; });',
        '',
        '    completeBtn.addEventListener("click", async function(){',
        '      var params=new URLSearchParams(location.search); var rid=params.get("rid")||params.get("RID")||"";',
        '      if(!rid){ Swal.fire({title:"알림",text:"RID가 없어 수강 완료를 기록할 수 없습니다.",icon:"warning",confirmButtonText:"확인"}); return; }',
        '      var dur=Math.floor(v.duration||0); var watched=Math.floor(v.currentTime||0); var percent=dur?Math.round((watched/dur)*100):0;',
        '      if(percent<90){ Swal.fire({title:"알림",text:"영상 시청이 충분하지 않습니다. (90% 이상 시청 필요)",icon:"info",confirmButtonText:"확인"}); return; }',
        '      completeBtn.disabled=true;',
        '      fetch(TRAINING_COMPLETE_URL,{method:"POST",headers:{"Content-Type":"application/json"},credentials:"same-origin",body:JSON.stringify({rid:rid,video_id:(typeof videoId==="string"?videoId:Number(videoId)),duration:dur,watched:watched,percent:percent})})',
        '      .then(function(res){ if(!res.ok) return res.text().then(function(t){throw new Error(t);}); return res.json(); })',
        '      .then(function(){ Swal.fire({title:"수강 완료",text:"수강 완료 확인되었습니다. 감사합니다!",icon:"success",confirmButtonText:"확인"}).then(function(){ window.close(); }); })',
        '      .catch(function(err){ Swal.fire({title:"오류",text:"수강 기록 실패: "+(err&&err.message?err.message:err),icon:"error",confirmButtonText:"확인"}); completeBtn.disabled=false; });',
        '    });',
        '',
        '    window.addEventListener("beforeunload", function(){ beaconFlush(false); });',
        '    document.addEventListener("visibilitychange", function(){ if(document.visibilityState==="hidden") beaconFlush(false); });',
        '    if(saveTimer)clearInterval(saveTimer); saveTimer=setInterval(function(){ if(!v.paused&&!v.seeking&&v.readyState>=1)sendProgress(); },SAVE_INTERVAL_MS);',
        '  })();',
        '  <\/script>',
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
        successFlash("학습용 템플릿을 삽입했습니다.");
    } catch (e) {
        console.error(e);
        modalError("템플릿 삽입에 실패했습니다.");
    }
}
