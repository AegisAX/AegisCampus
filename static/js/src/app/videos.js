/*
  static/js/src/app/videos.js
  Grid + inline playback + kebab menu + clean upload UX (progress & abort)
*/

var videos = [];
var currentPlayingId = null;   // 인라인 재생 중인 카드 id
var uploading = false;         // 업로드 진행 중 여부
var currentUploadXhr = null;   // 진행 중인 jqXHR (abort 용)

function escapeHtml(s){ if(!s) return ''; return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;'); }
function showLoading(show){ $("#loading").toggle(!!show); $("#videos-grid").toggle(!show); }

function loadVideos(){
  showLoading(true);
  api.videos.get()
    .success(function(data){
      if(!Array.isArray(data)){ errorFlash("Unexpected response from server."); showLoading(false); return; }
      videos = data;
      renderVideos();
      showLoading(false);
    })
    .error(function(xhr){
      showLoading(false);
      if (xhr.status === 401) errorFlash("Unauthorized. Please refresh and sign in again.");
      else errorFlash("Error fetching videos");
    });
}

function formatDuration(seconds){
  seconds = Number(seconds || 0);
  var h = Math.floor(seconds/3600), m=Math.floor((seconds%3600)/60), s=seconds%60;
  if(h>0) return [h,String(m).padStart(2,'0'),String(s).padStart(2,'0')].join(":");
  return [m,String(s).padStart(2,'0')].join(":");
}
function thumbUrl(id){ return "/videos/thumb/"+id; }
function streamUrl(id){ return "/videos/stream/"+id; }

function renderVideos(){
  var $grid = $("#videos-grid");
  $grid.empty();

  if(!videos || videos.length===0){
    $("#emptyMessage").show();
    $grid.hide();
    return;
  }
  $("#emptyMessage").hide();

  videos.forEach(function(v){
    var len = formatDuration(v.duration_seconds);
    var uploadedAbs = v.modified_date ? moment(v.modified_date).format('YYYY-MM-DD HH:mm:ss') : '';
    var uploadedRel = v.modified_date ? moment(v.modified_date).fromNow() : '';

    // ── 공개 배지 ──────────────────────────────────────────
    var publicBadge = v.is_public
        ? ' <span class="badge-public">공개</span>'
        : ' <span class="badge-private">비공개</span>';

    // ── 소유자 아닌 경우 출처 표시 ─────────────────────────
    var sharedNote = (!v.is_owner)
        ? '<div class="video-shared"><i class="fa fa-share-alt"></i> 공유된 영상</div>'
        : '';

    // ── 케밥 메뉴: 소유자만 표시 ──────────────────────────
    var kebab = '';
    if (v.is_owner) {
        kebab =
          '<div class="kebab dropdown">' +
            '<button class="btn btn-default btn-xs dropdown-toggle" type="button" data-toggle="dropdown" aria-haspopup="true" aria-expanded="false" title="More">' +
              '<i class="fa fa-ellipsis-h"></i>' +
            '</button>' +
            '<ul class="dropdown-menu dropdown-menu-right">' +
              '<li><a onclick="editVideo('+v.id+'); return false;"><i class="fa fa-pencil"></i> 편집</a></li>' +
              '<li role="separator" class="divider"></li>' +
              '<li><a class="text-danger" onclick="deleteVideo('+v.id+'); return false;"><i class="fa fa-trash"></i> 삭제</a></li>' +
            '</ul>' +
          '</div>';
    }

    var thumb = (currentPlayingId === v.id)
      ? (
        '<div class="video-thumb playing" onclick="toggleInlinePlay('+v.id+')">' +
          '<video id="player-'+v.id+'" controls autoplay muted playsinline poster="'+thumbUrl(v.id)+'">' +
            '<source src="'+streamUrl(v.id)+'" type="video/mp4">' +
            'Your browser does not support the video tag.' +
          '</video>' +
        '</div>'
      )
      : (
        '<div class="video-thumb" onclick="toggleInlinePlay('+v.id+')">' +
          '<img src="'+thumbUrl(v.id)+'" alt="thumb" loading="lazy" onerror="this.style.display=\'none\'">' +
          '<span class="duration">'+len+'</span>' +
          '<span class="overlay"><i class="fa fa-play"></i></span>' +
        '</div>'
      );

    // ── 배지를 날짜와 같은 줄로, 날짜는 YYYY-MM-DD ──────────
    var uploadedDate = v.modified_date
      ? moment(v.modified_date).format('YYYY-MM-DD')
      : '';

    var metaHead =
      '<div class="meta-head">' +
        '<div class="video-title" title="'+escapeHtml(v.name)+'">'+escapeHtml(v.name)+'</div>' +
        kebab +                           // ← 배지 제거
      '</div>';

    // 배지와 날짜 같은 줄
    var sub = '<div class="video-sub">' + publicBadge +
              (uploadedDate ? '&nbsp;&nbsp;' + uploadedDate : '') +
              '</div>';

    var desc = v.description ? '<div class="video-desc">'+escapeHtml(v.description)+'</div>' : '';

    var card =
      '<div class="video-card">' +
        thumb +
        '<div class="video-meta">' +
          metaHead + sharedNote + sub + desc +   // ← sharedNote 추가
        '</div>' +
      '</div>';

    $grid.append(card);
  });

  $grid.show();

  if(currentPlayingId !== null){
    var el = document.getElementById('player-'+currentPlayingId);
    if(el){ el.play().catch(function(){}); }
  }
}

/* === Inline play toggle === */
function toggleInlinePlay(id){
  id = Number(id);
  currentPlayingId = (currentPlayingId === id) ? null : id;
  renderVideos();
}

/* === Edit === */
function editVideo(id){
  var found = getVideoAndIndexById(id);
  var v = found.v;
  if(!v){ errorFlash && errorFlash("대상 영상을 찾을 수 없습니다."); return; }

  var newTitle = window.prompt("제목", v.name || "");
  if(newTitle === null) return;
  newTitle = newTitle.trim();

  var newDesc = window.prompt("설명", v.description || "");
  if(newDesc === null) return;

  // 공개 여부
  var currentPublic = v.is_public ? "공개" : "비공개";
  var newPublicStr = window.prompt("공개 여부를 입력하세요 (공개 / 비공개)", currentPublic);
  if(newPublicStr === null) return;
  var newIsPublic = newPublicStr.trim() === "공개";

  var payload = {
    id: v.id, user_id: v.user_id,
    name: newTitle, description: newDesc,
    file_name: v.file_name, file_path: v.file_path,
    thumbnail_path: v.thumbnail_path, duration_seconds: v.duration_seconds,
    is_public: newIsPublic
  };

  api.videoId.put(payload)
    .success(function(){
      v.name = newTitle;
      v.description = newDesc;
      v.is_public = newIsPublic;
      renderVideos();
      successFlash && successFlash("수정되었습니다.");
    })
    .error(function(xhr){
      var msg = (xhr.responseJSON && xhr.responseJSON.message) || "수정 실패";
      errorFlash && errorFlash(msg);
    });
}

/* === Delete === */
function getVideoAndIndexById(id){
  id = Number(id);
  for(var i=0;i<videos.length;i++){
    if(Number(videos[i].id) === id) return { v: videos[i], idx: i };
  }
  return { v:null, idx:-1 };
}

function deleteVideo(id){
  var found = getVideoAndIndexById(id);
  var v = found.v, idx = found.idx;
  if(!v){ errorFlash && errorFlash("대상 영상을 찾을 수 없습니다."); return; }
  if(!confirm("'" + ((v.name||"이 영상")) + "'을(를) 삭제할까요?")) return;

  api.videoId.delete(v.id)
    .success(function(){
      if(currentPlayingId === v.id) currentPlayingId = null;
      videos.splice(idx,1);
      renderVideos();
      successFlash && successFlash("삭제되었습니다.");
    })
    .error(function(xhr){
      var msg = (xhr.responseJSON && xhr.responseJSON.message) || "삭제 실패";
      errorFlash && errorFlash(msg);
    });
}

/* === Upload UX === */
function startUploadUI(){
  uploading = true;
  $("#btn-start-upload").prop("disabled", true);
  $("#btn-cancel-upload").text("Cancel Upload").prop("disabled", false);
  $("#upload-status").show().text("Uploading…");
  $("#upload-progress").show();
  $("#upload-progress-bar").css("width","0%").attr("aria-valuenow",0).text("0%");
}
function setUploadProgress(pct){
  pct = Math.max(0, Math.min(100, pct|0));
  $("#upload-progress-bar").css("width", pct+"%").attr("aria-valuenow", pct).text(pct+"%");
  $("#upload-status").text(pct>=100 ? "Processing…" : ("Uploading… " + pct + "%"));
}
function endUploadUI(){
  uploading = false;
  currentUploadXhr = null;
  $("#btn-start-upload").prop("disabled", false);
  $("#btn-cancel-upload").text("Cancel").prop("disabled", false);
  $("#upload-status").hide().text("");
  $("#upload-progress").hide();
}
function resetUploadForm(){
  $("#video-upload-form")[0].reset();
  $("#video-duration-seconds").val("0");
}

/* 업로드 (XHR progress + abort 지원) */
function wireUploadForm(){
  $("#video-upload-form").on("submit", function(e){
    e.preventDefault();

    var fd = new FormData(this);

    startUploadUI();

    currentUploadXhr = $.ajax({
      url: "/videos/upload",
      method: "POST",
      data: fd,
      contentType: false,
      processData: false,
      headers: {
        // gorilla/csrf: AJAX는 X-CSRF-Token 헤더가 필요
        "X-CSRF-Token": $("#video-upload-form input[name='csrf_token']").val()
      },
      xhr: function(){
        var xhr = $.ajaxSettings.xhr();
        if (xhr.upload) {
          xhr.upload.addEventListener("progress", function(evt){
            if (evt.lengthComputable) {
              var pct = Math.round((evt.loaded / evt.total) * 100);
              setUploadProgress(pct);
            }
          }, false);
        }
        return xhr;
      },
      success: function(v){
        endUploadUI();
        $("#uploadModal").modal("hide");
        resetUploadForm();

        // 성공 즉시 최상단에 아이템 추가 → 깜빡임 없이 보여줌
        if (v && v.id) {
          v.is_owner = true;  // 업로드한 사람이 소유자
          videos.unshift(v);
          renderVideos();
        } else {
          // 혹시 모를 구조 변화 대비
          loadVideos();
        }
        successFlash && successFlash("Uploaded!");
      },
      error: function(xhr, status){
        endUploadUI();
        if (status === "abort") {
          errorFlash && errorFlash("Upload canceled.");
          return;
        }
        var msg = (xhr.responseJSON && xhr.responseJSON.message) || "Upload failed";
        (typeof modalError==="function" ? modalError : errorFlash)(msg);
      }
    });
  });

  // 취소 버튼: 업로드 중이라면 즉시 중단
  $("#btn-cancel-upload").on("click", function(){
    if (uploading && currentUploadXhr) {
      try { currentUploadXhr.abort(); } catch(e){}
    }
  });

  // 모달 닫힐 때 UI 원복 (혹시 외부 닫힘 케이스)
  $('#uploadModal').on('hidden.bs.modal', function () {
    if (uploading && currentUploadXhr) {
      try { currentUploadXhr.abort(); } catch(e){}
    }
    endUploadUI();
    // 폼은 닫힐 때 깔끔히
    // resetUploadForm();  // 필요시 활성화
  });
}

/* 파일 선택 시: 제목 자동 채움 + 길이 추출 */
function wireDurationAndTitle(){
  $("#video-file").on("change", function(){
    var file = this.files && this.files[0];
    var $title = $("#video-name");
    if (file && $title.length && !$title.val().trim()) {
      var name = (file.name||"").split('/').pop().split('\\').pop();
      var dot = name.lastIndexOf('.');
      if (dot>0) name = name.substring(0,dot);
      name = name.replace(/[_\-]+/g,' ').replace(/\s+/g,' ').trim();
      $title.val(name).trigger("input");
    }
    var $hidden = $("#video-duration-seconds");
    $hidden.val("0");
    if (!file) return;

    try{
      var url = URL.createObjectURL(file);
      var vid = document.createElement("video");
      vid.preload = "metadata"; vid.muted = true; vid.src = url;
      vid.onloadedmetadata = function(){
        var dur = Math.max(0, Math.round(vid.duration || 0));
        $hidden.val(String(dur));
        URL.revokeObjectURL(url);
        vid.removeAttribute("src");
        try{ vid.load(); }catch(e){}
      };
      vid.onerror = function(){
        URL.revokeObjectURL(url);
        vid.removeAttribute("src");
        try{ vid.load(); }catch(e){}
      };
    }catch(e){}
  });
}

/* === init === */
$(document).ready(function(){
  showLoading(true);
  wireUploadForm();
  wireDurationAndTitle();
  loadVideos();
});

