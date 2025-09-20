/*
  static/js/src/app/videos.js
  Video Management (list, upload, delete, preview)
  Author: adapted for Gophish UI
*/

var videos = [];

function escapeHtml(s) {
  if (!s) return '';
  return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
}

function showLoading(show) {
  if (show) {
    $("#videos-table").hide();
    $("#loading").show();
  } else {
    $("#loading").hide();
    $("#videos-table").show();
  }
}

/* === List === */
function loadVideos() {
  showLoading(true);
  // use api.videos.get() (defined in gophish.js)
  api.videos.get()
    .success(function (data) {
      if (!Array.isArray(data)) {
        errorFlash("Unexpected response from server.");
        showLoading(false);
        return;
      }
      videos = data;
      renderVideos();
      showLoading(false);
    })
    .error(function (xhr) {
      showLoading(false);
      if (xhr.status === 401) {
        errorFlash("Unauthorized. Please refresh and sign in again.");
      } else {
        errorFlash("Error fetching videos");
      }
    });
}

function renderVideos() {
  var $tbody = $("#videos-body");
  $tbody.empty();
  if (!videos || videos.length === 0) {
    // 테이블은 숨기고 빈 상태로 둠 (템플릿에 #emptyMessage가 있으면 이를 활용하도록 확장 가능)
    $("#videos-table").show();
    return;
  }
  videos.forEach(function(v, i){
    var tr = [
      "<tr>",
      "<td>", escapeHtml(v.name), "</td>",
      "<td>", escapeHtml(v.description || ""), "</td>",
      "<td>", v.modified_date ? moment(v.modified_date).format('MMMM Do YYYY, h:mm:ss a') : "", "</td>",
      "<td class='text-right'>",
        "<button class='btn btn-primary btn-xs' data-toggle='tooltip' title='Preview' onclick='previewVideo(", v.id, ")'>",
          "<i class='fa fa-play'></i>",
        "</button> ",
        "<button class='btn btn-danger btn-xs' data-toggle='tooltip' title='Delete' onclick='deleteVideo(", i, ")'>",
          "<i class='fa fa-trash-o'></i>",
        "</button>",
      "</td>",
      "</tr>"
    ].join("");
    $tbody.append(tr);
  });
  $('[data-toggle="tooltip"]').tooltip();
}

/* === Preview === */
function previewVideo(id) {
  // Set source to stream endpoint and show modal
  var src = "/videos/stream/" + id;
  $("#preview-src").attr("src", src);
  var v = document.getElementById("preview-video");
  try {
    // reload the media element so the new src is picked up
    v.load();
  } catch (e) {
    console.error("Video load error:", e);
  }
  $("#previewModal").modal("show");
}

/* When preview modal is closed, ensure playback fully stops and source is released */
function wirePreviewStopOnClose() {
  $('#previewModal').on('hidden.bs.modal', function () {
    var v = document.getElementById("preview-video");
    if (v) {
      try {
        // Pause, reset time, clear source and unload to free resources and stop network
        v.pause();
        try { v.currentTime = 0; } catch (e) { /* 일부 브라우저에서 예외 가능 */ }
        $("#preview-src").attr("src", "");
        v.load();
      } catch (err) {
        console.error("Error while stopping preview video:", err);
      }
    }
  });
}

/* === Delete === */
function deleteVideo(idx) {
  var vid = videos[idx];
  Swal.fire({
    title: "Are you sure?",
    text: "This will delete the video. This can't be undone!",
    type: "warning",
    animation: false,
    showCancelButton: true,
    confirmButtonText: "Delete " + escapeHtml(vid.name),
    confirmButtonColor: "#428bca",
    reverseButtons: true,
    allowOutsideClick: false,
    preConfirm: function () {
      return new Promise(function (resolve, reject) {
        api.videoId.delete(vid.id)
          .success(function(){ resolve(); })
          .error(function(xhr){
            var msg = (xhr.responseJSON && xhr.responseJSON.message) || "Delete failed";
            reject(msg);
          });
      });
    }
  }).then(function (result) {
    if (result.value){
      Swal.fire('Video Deleted!', 'The video has been deleted!', 'success');
      loadVideos();
    }
  });
}

/* === Upload === */
function wireUploadForm() {
  $("#video-upload-form").on("submit", function (e) {
    e.preventDefault();
    var fd = new FormData(this);
    // ensure the checkbox value is explicitly present
    var isPublic = $("#video-public").prop("checked") ? "true" : "false";
    fd.set("is_public", isPublic);

    showLoading(true);
    api.videos.post(fd)
      .success(function (v) {
        $("#uploadModal").modal("hide");
        $("#video-upload-form")[0].reset();
        successFlash("Video uploaded successfully!");
        loadVideos();
      })
      .error(function (xhr) {
        showLoading(false);
        var msg = (xhr.responseJSON && xhr.responseJSON.message) || "Upload failed";
        modalError(msg);
      });
  });
}

/* === init === */
$(document).ready(function(){
  // 초기 로드 UX
  showLoading(true);
  wireUploadForm();
  wirePreviewStopOnClose();
  loadVideos();
});

