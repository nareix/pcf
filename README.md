
使用
----

		import "gitcafe.com/nuomi-studio/pcf.git"

		func main() {
			if f, err := pcf.Open("wenquanyi_13px.pcf"); err == nil {
				f.DumpAscii("out", '操')
			}
		}

输出
	
	$ cat out
 
			 *   *****                     
			 *   *   *                     
		 ***** *****                     
			 *                             
			 *  *** ***                    
			 * ** * * *                    
			 ** *** ***                    
		 ***     *                       
			 * *********                   
			 *    ***                      
			 *   * * *                     
		 * *  *  *  *                    
			*  *   *   *                   

