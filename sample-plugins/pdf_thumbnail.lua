-- requires ghostscript to be installed
local os = require("os")

local cmd = "gs -q -sDEVICE=jpeg -dLastPage=1 -dNOPAUSE -r720 -g%dx%d -dPDFFitPage -dFIXEDMEDIA -dCompatibilityLevel=1.4 -o %q - <  %q" -- width, height outpath, inpath

register_upload_handler(".pdf", function(upload, post, board, filePath, thumbPath, catalogThumbPath, infoEv, accessEv, errEv)
	-- width, height = get_pdf_dimensions(filePath)
	local boardcfg = board_config(board)
	upload.ThumbnailWidth = boardcfg.ThumbWidthReply
	upload.ThumbnailHeight = boardcfg.ThumbHeightReply
	if (post.IsTopPost) then
		upload.ThumbnailWidth = boardcfg.ThumbWidth
		upload.ThumbnailHeight = boardcfg.ThumbHeight
		status = os.execute(string.format(cmd, boardcfg.ThumbWidthCatalog, boardcfg.ThumbHeightCatalog, catalogThumbPath, filePath))
		if (status ~= 0) then
			return "unable to create PDF catalog thumbnail"
		end
	end
	
	status = os.execute(string.format(cmd, upload.ThumbnailWidth, upload.ThumbnailHeight, thumbPath, filePath))
	if (status ~= 0) then
		return "unable to create PDF thumbnail"
	end

	return nil
end)
set_thumbnail_ext(".pdf", ".jpg")