def are_files_equal(file1_path, file2_path):
    try:
        with open(file1_path, 'rb') as file1, open(file2_path, 'rb') as file2:
            while True:
                chunk1 = file1.read(4096)
                chunk2 = file2.read(4096)
                
                if chunk1 != chunk2:
                    return False
                
                if not chunk1:
                    return True
    except FileNotFoundError:
        return False


print(are_files_equal('easytest', 'testfile1'))  # False
print(are_files_equal('beemovie', 'testfile2'))  # True
