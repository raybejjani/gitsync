#!/usr/bin/env python
"""builds a Go file with a map of file paths to content"""

import os,hashlib,argparse,time, sys

def get_file_tuples(path=".", base_dir=None, walker=None):
    """returns a list of tuples (file path, file content) for a recursive walk of path"""
    base_dir = path if base_dir == None else base_dir
    walker = os.walk(path, topdown=True, followlinks=True) if walker == None else walker
    accum = []

    for dirname, dirnames, filenames in walker:
        for filename in filenames:
            # on disk path
            file_path = os.path.join(dirname, filename)
            # on server path: relative path to base dir
            http_path = file_path[len(base_dir):]
            # read in file data
            f = open(file_path, 'ro')
            try:
                file_data = f.read()
            finally:
                f.close()
        
            accum.append((http_path, file_data))
        
        for subdirname in dirnames:
            accum.extend(get_file_tuples(path=os.path.join(dirname, subdirname), 
                base_dir=base_dir, walker=walker))
    return accum


def generate_file(template, files, index_filename, gencmd):
    """generate a Go file with a map of file paths to content with an additional
    entry for the root index, '/' """
    # code blocks to define the data
    data_blocks = []

    for file, data in files:
        data_str = "".join(["%0.2x"%(ord(b)) for b in data])
        data_blocks.append('"%s": `%s`'%(file, data_str))
        if index_filename == file:
            data_blocks.append('"/": `%s`'%(data_str))
    
    ret = template
    ret = ret.replace("// +build !makebuild", "// +build makebuild")
    ret = ret.replace("//###DATA###", ",\n".join(data_blocks))
    ret = ret.replace("###GENDATE###", time.ctime())
    ret = ret.replace("###GENCMD###", gencmd)
    return ret

def parseArgs():
    parser = argparse.ArgumentParser(description="make_code_fs: Builder for content files")
    parser.add_argument('root', type=str, 
        help="path to root directory of files to be served")
    parser.add_argument('--index', dest='index_name', default="/index.html", type=str, 
        help="server path of file that is the root")
    parser.add_argument('-i' ,'--template', dest='template_file', 
        default="template.go", type=str, 
        help="template file to be used to generate ragel matcher/content file")
    parser.add_argument('-o', dest='output_name', default="", type=str, 
        help="path of output file")

    options,unknownArgs = parser.parse_known_args()
    return options,unknownArgs

if __name__ == "__main__":
    options, unknownArgs = parseArgs()

    tmpl_f = open(options.template_file, 'ro')
    tmpl = None
    try:
        tmpl = tmpl_f.read()
    finally:
        tmpl_f.close()

    file_info = get_file_tuples(path=options.root)
    sys.stderr.write("Preparing:%s"%([x[0] for x in file_info]))
    output = generate_file(tmpl, file_info, options.index_name, 
                           " ".join(sys.argv))

    output_f = sys.stdout
    if options.output_name != "":
        output_f = open(options.output_name, "w+")

    try:
        output_f.write(output)
    finally:
        output_f.close()





