package share

import (
	"html/template"
	"time"
)

func defaultExpire() string {
	return time.Now().Add(24 * time.Hour).Format(timeLayout)
}

var tmplList = template.Must(template.New("list").Funcs(template.FuncMap{"defaultExpire": defaultExpire}).Parse(`<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Strict//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-strict.dtd">
<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="en" lang="en">
<head>
    <meta http-equiv="Content-Type" content="text/html; charset=utf-8" />
    <title>go-share</title>
    <style type="text/css">
        td.left
        {
			text-align: right;
			margin-right: 2px
        }
		td.right
		{
			text-align: left
		}
    </style>
</head>
<body>
	<p>
		{{range .}}
		<a href="{{.}}">{{.}}</a><br />
		{{end}}
	</p>
    <form action="" method="post" enctype="multipart/form-data">
		<table id="uploadTable">
			<tr>
				<td class="left">File:</td>
				<td class="right"><input type="file" name="file" /></td>
			</tr>
			<tr>
				<td class="left">Password:</td>
				<td class="right"><input type="password" name="password" /></td>
			</tr>
			<tr>
				<td class="left">File name:</td>
				<td class="right">
					<input type="text" name="name" />
					(blank means use the original one)
				</td>
			</tr>
			<tr>
				<td class="left">Expires:</td>
				<td class="right">
					<input type="text" name="expire" value="{{defaultExpire}}" />
					(blank means never)
				</td>
			</tr>
			<tr>
				<td class="left">Maximum number of downloads:</td>
				<td class="right">
					<input type="text" name="downloads" value="" />
					(blank means unlimited)
				</td>
			</tr>
			<tr>
				<td class="left">Public:</td>
				<td class="right">
					<input type="checkbox" name="public" value="public" />
				</td>
			</tr>
			<tr>
				<td class="left">Delete from server when becomes unaccessible:</td>
				<td class="right">
					<input type="checkbox" name="delete" value="delete" />
				</td>
			</tr>
			<tr>
				<td class="left"></td>
				<td class="right">
					<input type="submit" name="upload" value="Upload" />
				</td>
			</tr>
		</table>
    </form>
	<p id="footer">
	</p>
	<script type="text/javascript">
		document.getElementById("uploadTable").style.display = "none";
		link = document.createElement("a");
		link.setAttribute("href", "#");
		link.setAttribute("onclick", "this.style.display = 'none'; document.getElementById('uploadTable').style.display = 'table'; return false;");
		link.appendChild(document.createTextNode("Upload"));
		document.getElementById("footer").appendChild(link);
	</script>
</body>
</html>`))
