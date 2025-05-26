bl_info = {
    "name": "Export Markers as YAML Chapters",
    "blender": (2, 80, 0),
    "category": "Export",
    "version": (1, 0, 0),
    "author": "OpenAI + SA6MWA",
    "description": "Exports timeline markers to a YAML file formatted as video chapters.",
}

import bpy
import os

class EXPORT_OT_markers_to_yaml(bpy.types.Operator):
    """Export Timeline Markers to a YAML Chapter File"""
    bl_idname = "export.markers_to_yaml"
    bl_label = "Export Markers as YAML Chapters"
    bl_options = {'REGISTER'}

    def execute(self, context):
        output_path = bpy.path.abspath("//chapters.yaml")
        fps = context.scene.render.fps
        start_frame = context.scene.frame_start
        markers = sorted(context.scene.timeline_markers, key=lambda m: m.frame)

        try:
            with open(output_path, "w", encoding="utf-8") as f:
                f.write("chapters:\n")
                for marker in markers:
                    time_seconds = (marker.frame - start_frame) / fps
                    hours = int(time_seconds // 3600)
                    minutes = int((time_seconds % 3600) // 60)
                    seconds = int(time_seconds % 60)
                    milliseconds = int((time_seconds - int(time_seconds)) * 1000)
                    timestamp = f"{hours:02d}:{minutes:02d}:{seconds:02d}.{milliseconds:03d}"
                    f.write(f"- title: \"{marker.name}\"\n")
                    f.write(f"  start: \"{timestamp}\"\n")
        except Exception as e:
            self.report({'ERROR'}, f"Failed to write file: {e}")
            return {'CANCELLED'}

        self.report({'INFO'}, f"Markers exported to: {output_path}")
        return {'FINISHED'}

def menu_func(self, context):
    self.layout.operator(EXPORT_OT_markers_to_yaml.bl_idname)

def register():
    bpy.utils.register_class(EXPORT_OT_markers_to_yaml)
    bpy.types.TOPBAR_MT_file_export.append(menu_func)

def unregister():
    bpy.types.TOPBAR_MT_file_export.remove(menu_func)
    bpy.utils.unregister_class(EXPORT_OT_markers_to_yaml)

if __name__ == "__main__":
    register()

